package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/ws"
)

var ErrDeleteConfirmationInvalid = errors.New("invalid or expired delete confirmation token")

const (
	keepaliveLeaseID          = "ibmon-keepalive"
	keepaliveLeaseTTLSeconds  = 3600
	keepaliveRenewLeadSeconds = 2400 // renew when <=40m left on 60m TTL (~every 20m)
	keepaliveRestartCapMaxSec = 25200
)

type keepaliveCtrl struct {
	toggleMu  sync.Mutex
	mu        sync.Mutex
	gen       uint64
	inFlight  bool
	cancelled bool
	cancel    context.CancelFunc
	done      chan struct{}
}

type Poller struct {
	store         *store.Store
	client        *instabox.Client
	interval      time.Duration
	retentionDays int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	accountLocks           sync.Map // key: accountID -> chan struct{} (cap 1)
	clockOffsetMs          atomic.Int64
	lastPruneUnix          atomic.Int64
	lastUsageCheck         sync.Map // key: accountID+":"+machineID -> int64 unix
	inactiveMetricDone     sync.Map // key: accountID+":"+machineID -> bool
	keepaliveCtrls         sync.Map // key: accountID+":"+machineID -> *keepaliveCtrl
	keepaliveLeaseRunStart sync.Map // key: accountID+":"+machineID -> string
	keepaliveDelayOverride atomic.Int64

	deleteChallengeMu sync.Mutex
	deleteChallenges  map[string]deleteChallengeEntry
}

type deleteChallengeEntry struct {
	accountID   string
	machineID   string
	machineName string
	expiresAt   time.Time
}

type DeleteConfirmationChallenge struct {
	RequiresUserConfirmation bool   `json:"requiresUserConfirmation"`
	ConfirmationToken        string `json:"confirmationToken"`
	AccountID                string `json:"accountId"`
	AccountAlias             string `json:"accountAlias,omitempty"`
	MachineID                string `json:"machineId"`
	MachineName              string `json:"machineName"`
	ExpiresInSeconds         int    `json:"expiresInSeconds"`
	Instruction              string `json:"instruction"`
}

func (p *Poller) accountLockChan(accountID string) chan struct{} {
	chIface, _ := p.accountLocks.LoadOrStore(accountID, make(chan struct{}, 1))
	return chIface.(chan struct{})
}

func (p *Poller) lockAccount(accountID string) func() {
	ch := p.accountLockChan(accountID)
	ch <- struct{}{}
	return func() { <-ch }
}

func (p *Poller) lockAccountContext(ctx context.Context, accountID string) (func(), bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	ch := p.accountLockChan(accountID)
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, true
	case <-ctx.Done():
		return nil, false
	}
}

func NewPoller(st *store.Store, client *instabox.Client, interval time.Duration, retentionDays int) *Poller {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	if retentionDays < 1 {
		retentionDays = 7
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Poller{
		store:         st,
		client:        client,
		interval:      interval,
		retentionDays: retentionDays,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (p *Poller) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.SyncAll(p.ctx)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				p.SyncAll(p.ctx)
			}
		}
	}()
}

func (p *Poller) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (p *Poller) ClockOffsetMs() int64 {
	return p.clockOffsetMs.Load()
}

func (p *Poller) SyncAll(ctx context.Context) {
	accounts, err := p.store.ListAccounts(ctx)
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := p.SyncAccount(ctx, id); err != nil && ctx.Err() == nil {
				log.Printf("sync account %s: %v", id, err)
			}
		}(acc.ID)
	}
	wg.Wait()

	now := time.Now().Unix()
	if now-p.lastPruneUnix.Load() >= 3600 {
		p.lastPruneUnix.Store(now)
		cutoff := now - int64(p.retentionDays)*86400
		if _, err := p.store.PruneMetricsBefore(ctx, cutoff); err != nil && ctx.Err() == nil {
			log.Printf("prune metrics: %v", err)
		}
	}
}

func (p *Poller) SyncAccount(ctx context.Context, accountID string) error {
	unlock := p.lockAccount(accountID)
	defer unlock()

	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return err
	}
	me, meErr := p.client.GetMe(ctx, apiKey)
	machines, srvTime, mErr := p.client.ListAllMachines(ctx, apiKey)
	if !srvTime.IsZero() {
		p.clockOffsetMs.Store(srvTime.Sub(time.Now().UTC()).Milliseconds())
	}

	meta := store.AccountInput{}
	if meErr == nil && me != nil {
		meta.InstaboxUserID = me.UserID
		meta.InstaboxLogin = me.Login
		if me.Org != nil {
			meta.OrgID = me.Org.ID
			meta.OrgName = me.Org.Name
			meta.OrgTier = me.Org.Tier
		}
	}
	if usageRaw, uErr := p.client.GetUsageRaw(ctx, apiKey); uErr == nil && len(usageRaw) > 0 {
		meta.OrgUsageJSON = string(usageRaw)
	}

	if mErr != nil {
		_ = p.store.UpdateAccountSyncMeta(ctx, accountID, meta, mErr.Error())
		return mErr
	}

	existingList, _ := p.store.ListMachines(ctx)
	existingMap := make(map[string]store.MachineRecord)
	for _, em := range existingList {
		if em.AccountID == accountID {
			existingMap[em.MachineID] = em
		}
	}

	records := make([]store.MachineRecord, 0, len(machines))
	for _, m := range machines {
		rec := toMachineRecord(accountID, m)
		if prev, ok := existingMap[m.ID]; ok {
			rec.MonthlyRunningSeconds = prev.MonthlyRunningSeconds
			rec.DiskJSON = prev.DiskJSON
		}
		records = append(records, rec)
	}

	if err := p.store.SyncAccountMachines(ctx, accountID, records); err != nil {
		return err
	}
	_ = p.store.UpdateAccountSyncMeta(ctx, accountID, meta, "")

	for _, m := range machines {
		prev, hadPrev := existingMap[m.ID]
		prevState := ""
		if hadPrev {
			prevState = prev.State
		}
		p.syncMachineTelemetry(ctx, accountID, apiKey, m, prevState)
		if hadPrev && prev.KeepaliveEnabled {
			if p.needsKeepaliveAction(m, prev) {
				p.triggerKeepalive(accountID, m.ID, false)
			} else {
				p.reconcileSkippedKeepalive(ctx, accountID, m, prev)
			}
		}
		if hadPrev && prev.ScheduleEnabled {
			p.enforceSchedule(ctx, accountID, apiKey, m, prev)
		}
	}
	return nil
}

func (p *Poller) syncMachineTelemetry(ctx context.Context, accountID, apiKey string, m instabox.Machine, prevState string) {
	now := time.Now()
	usageKey := accountID + ":" + m.ID
	var monthlySec int64 = -1

	// Fetch monthly usage immediately on first sight or every 2 minutes while running (every 10m otherwise).
	lastU, hasU := p.lastUsageCheck.Load(usageKey)
	usageInterval := int64(600)
	if m.State == "running" {
		usageInterval = 90
	}
	if !hasU || now.Unix()-lastU.(int64) >= usageInterval {
		if u, err := p.client.GetMachineUsage(ctx, apiKey, m.ID); err == nil && u != nil {
			monthlySec = u.RunningSeconds
			p.lastUsageCheck.Store(usageKey, now.Unix())
		}
	}

	// Determine metric query window: backfill ~23h45m if empty/old (strict <24h upstream limit),
	// else incremental since last sample - 90s.
	upstreamNow := now.Add(time.Duration(p.clockOffsetMs.Load()) * time.Millisecond)
	minSince := upstreamNow.Add(-23*time.Hour - 45*time.Minute)
	since := minSince
	existingMetrics, err := p.store.GetMachineMetrics(ctx, accountID, m.ID, now.Add(-48*time.Hour).Unix())
	hasAnyMetrics := err == nil && len(existingMetrics) > 0
	if hasAnyMetrics {
		lastTS := existingMetrics[len(existingMetrics)-1].TS
		if now.Unix()-lastTS < 23*3600 {
			cand := time.Unix(lastTS-90, 0).UTC()
			if cand.After(minSince) {
				since = cand
			}
		}
	}

	// Poll /metrics while running, on transition out of running/booting, or once when no metrics have been attempted yet.
	wasActive := prevState == "running" || prevState == "starting" || prevState == "resuming"
	_, alreadyFinalized := p.inactiveMetricDone.Load(usageKey)
	shouldPollMetrics := m.State == "running" || (wasActive && m.State != "running") || !alreadyFinalized
	if m.State == "running" {
		p.inactiveMetricDone.Delete(usageKey)
	}

	var diskJSON string
	var diskPtr *instabox.MachineDisk
	var samples []store.MetricSample

	if shouldPollMetrics {
		mm, mErr := p.client.GetMachineMetrics(ctx, apiKey, m.ID, since)
		if mErr != nil {
			// Retry once with upstream default window (last 5m) if `since` was rejected due to clock drift.
			var apiErr *instabox.APIError
			if errors.As(mErr, &apiErr) && apiErr.Status == 400 {
				mm, mErr = p.client.GetMachineMetrics(ctx, apiKey, m.ID, time.Time{})
			}
		}
		if mErr != nil && ctx.Err() == nil {
			log.Printf("metrics %s/%s: %v", accountID, m.ID, mErr)
		}
		if mErr == nil && mm != nil {
			diskPtr = mm.Disk
			if diskPtr == nil && m.State == "running" {
				if d, dErr := p.client.GetMachineDisk(ctx, apiKey, m.ID); dErr == nil && d != nil {
					diskPtr = d
				}
			}
			var dataUsed, dataTotal, rootUsed, rootTotal int64
			if diskPtr != nil {
				if rawDisk, jErr := json.Marshal(diskPtr); jErr == nil {
					diskJSON = string(rawDisk)
				}
				for _, fs := range diskPtr.Filesystems {
					switch fs.Mount {
					case "/data":
						dataUsed = fs.UsedBytes
						dataTotal = fs.SizeBytes
					case "/":
						rootUsed = fs.UsedBytes
						rootTotal = fs.SizeBytes
					}
				}
			}
			for i, pt := range mm.Metrics {
				t, parseErr := time.Parse(time.RFC3339, pt.Timestamp)
				if parseErr != nil {
					continue
				}
				s := store.MetricSample{
					AccountID:    accountID,
					MachineID:    m.ID,
					TS:           t.Unix(),
					CPUPct:       pt.CPUUsedPct,
					CPUCount:     pt.CPUCount,
					MemUsed:      pt.MemUsed,
					MemTotal:     pt.MemTotal,
					DiskUsed:     pt.DiskUsed,
					DiskTotal:    pt.DiskTotal,
					NetInBPS:     pt.NetInBytesPerSec,
					NetOutBPS:    pt.NetOutBytesPerSec,
					DiskReadBPS:  pt.DiskReadBytesPerSec,
					DiskWriteBPS: pt.DiskWriteBytesPerSec,
				}
				if i == len(mm.Metrics)-1 {
					if dataUsed > 0 {
						s.DiskUsed = dataUsed
					}
					if dataTotal > 0 {
						s.DiskTotal = dataTotal
					}
					s.RootUsed = rootUsed
					s.RootTotal = rootTotal
				}
				samples = append(samples, s)
			}
			if m.State != "running" {
				p.inactiveMetricDone.Store(usageKey, true)
			}
		} else if m.State == "running" {
			if d, dErr := p.client.GetMachineDisk(ctx, apiKey, m.ID); dErr == nil && d != nil {
				if rawDisk, jErr := json.Marshal(d); jErr == nil {
					diskJSON = string(rawDisk)
				}
			}
		}
	}

	if len(samples) > 0 {
		_ = p.store.UpsertMetrics(ctx, samples)
	}
	if diskJSON != "" || monthlySec >= 0 {
		_ = p.store.UpdateMachineDiskAndUsage(ctx, accountID, m.ID, diskJSON, monthlySec)
	}
}

func (p *Poller) ExecuteAction(ctx context.Context, accountID, machineID string, requested instabox.Action) (*instabox.ActionResult, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	m, err := p.store.GetMachine(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}

	effective := requested
	if requested == instabox.ActionWake && m.State == "suspended" && m.AutoSuspendedAt == "" {
		// Manual pause is resumed via /resume; idle auto-suspend is woken via /wake.
		effective = instabox.ActionResume
	}

	if requested == instabox.ActionStop {
		key := accountID + ":" + machineID
		p.cancelKeepaliveCycle(ctx, key, 2*time.Second)
		_ = p.store.SetMachineKeepalivePendingRestart(ctx, accountID, machineID, false)
		p.deleteKeepaliveLeaseRunStart(key)
	}

	res, err := p.client.PerformAction(ctx, apiKey, machineID, effective)
	if err != nil {
		return nil, err
	}

	if requested == instabox.ActionStop && m.KeepaliveEnabled {
		_ = p.store.UpdateMachineKeepaliveState(ctx, accountID, machineID, 0, time.Now().Unix(), "manual_paused", "")
	}

	if res.Machine != nil {
		_ = p.syncSingleMachine(ctx, accountID, *res.Machine)
	} else if res.Operation != nil && res.Operation.ID != "" {
		if res.Operation.Machine != nil {
			_ = p.syncSingleMachine(ctx, accountID, *res.Operation.Machine)
		}
		if res.Operation.State != "succeeded" && res.Operation.State != "failed" {
			p.trackOperation(accountID, apiKey, res.Operation.ID)
		}
	}
	return res, nil
}

func (p *Poller) syncSingleMachine(ctx context.Context, accountID string, m instabox.Machine) error {
	unlock, ok := p.lockAccountContext(ctx, accountID)
	if !ok {
		return ctx.Err()
	}
	defer unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	existing, err := p.store.ListMachines(ctx)
	if err != nil {
		return err
	}
	updated := make([]store.MachineRecord, 0, len(existing))
	found := false
	for _, r := range existing {
		if r.AccountID != accountID {
			continue
		}
		if r.MachineID == m.ID {
			rec := toMachineRecord(accountID, m)
			rec.MonthlyRunningSeconds = r.MonthlyRunningSeconds
			rec.DiskJSON = r.DiskJSON
			updated = append(updated, rec)
			found = true
		} else {
			updated = append(updated, r)
		}
	}
	if !found {
		updated = append(updated, toMachineRecord(accountID, m))
	}
	return p.store.SyncAccountMachines(ctx, accountID, updated)
}

type TemplateOption struct {
	ID          string               `json:"id"`
	Version     string               `json:"version"`
	Description string               `json:"description,omitempty"`
	Arch        string               `json:"arch"`
	DefaultSpec instabox.MachineSpec `json:"defaultSpec"`
	Default     bool                 `json:"default,omitempty"`
	PlanAllowed bool                 `json:"planAllowed"`
}

type AccountMachineOptions struct {
	AccountID               string                 `json:"accountId"`
	OrgTier                 string                 `json:"orgTier"`
	AllowedSpecs            []instabox.MachineSpec `json:"allowedSpecs"`
	ManualLifecycle         bool                   `json:"manualLifecycle"`
	AlwaysOnAllowed         bool                   `json:"alwaysOnAllowed"`
	FixedAutoSuspendSeconds int                    `json:"fixedAutoSuspendSeconds,omitempty"`
	IdleChoicesSeconds      []int                  `json:"idleChoicesSeconds,omitempty"`
	Templates               []TemplateOption       `json:"templates"`
}

func isFreeAllowedTemplate(id string) bool {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "agent-browser", "android", "omarchy", "ci":
		return true
	default:
		return false
	}
}

func (p *Poller) GetAccountMachineOptions(ctx context.Context, accountID string) (*AccountMachineOptions, error) {
	acc, err := p.store.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	me, err := p.client.GetMe(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	rawTemplates, err := p.client.ListTemplates(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	orgTier := acc.OrgTier
	if me.Org != nil && me.Org.Tier != "" {
		orgTier = me.Org.Tier
	}
	if orgTier == "" {
		orgTier = "free"
	}

	byID := make(map[string]instabox.Template)
	for _, tpl := range rawTemplates {
		creatable := !tpl.Hidden
		if tpl.Creatable != nil {
			creatable = *tpl.Creatable
		}
		if !creatable || tpl.GPU || tpl.Restricted {
			continue
		}
		if tpl.OS != "" && !strings.EqualFold(tpl.OS, "linux") {
			continue
		}
		prev, exists := byID[tpl.ID]
		if !exists || (tpl.Latest && !prev.Latest) || (tpl.Latest == prev.Latest) {
			byID[tpl.ID] = tpl
		}
	}

	templates := make([]TemplateOption, 0, len(byID))
	for _, tpl := range byID {
		planAllowed := !strings.EqualFold(orgTier, "free") || isFreeAllowedTemplate(tpl.ID)
		templates = append(templates, TemplateOption{
			ID:          tpl.ID,
			Version:     tpl.Version,
			Description: tpl.Description,
			Arch:        tpl.Arch,
			DefaultSpec: tpl.DefaultSpec,
			Default:     tpl.Default,
			PlanAllowed: planAllowed,
		})
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Default != templates[j].Default {
			return templates[i].Default
		}
		if templates[i].PlanAllowed != templates[j].PlanAllowed {
			return templates[i].PlanAllowed
		}
		return templates[i].ID < templates[j].ID
	})

	specs := me.AllowedSpecs
	if len(specs) == 0 {
		specs = []instabox.MachineSpec{{VCPUs: 4, MemMiB: 8192, PersistentDiskGiB: 20}}
	}

	return &AccountMachineOptions{
		AccountID:               accountID,
		OrgTier:                 orgTier,
		AllowedSpecs:            specs,
		ManualLifecycle:         me.ManualLifecycle,
		AlwaysOnAllowed:         me.AlwaysOnAllowed,
		FixedAutoSuspendSeconds: me.FixedAutoSuspendSeconds,
		IdleChoicesSeconds:      me.IdleChoicesSeconds,
		Templates:               templates,
	}, nil
}

func (p *Poller) CreateMachine(ctx context.Context, accountID string, in instabox.CreateMachineInput) (*instabox.ActionResult, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	res, err := p.client.CreateMachine(ctx, apiKey, in)
	if err != nil {
		return nil, err
	}
	syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = p.SyncAccount(syncCtx, accountID)

	if res.Operation != nil && res.Operation.ID != "" && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
		p.trackOperation(accountID, apiKey, res.Operation.ID)
	}
	return res, nil
}

func (p *Poller) RenameMachine(ctx context.Context, accountID, machineID, name string) (*store.MachineRecord, error) {
	return p.UpdateMachine(ctx, accountID, machineID, instabox.UpdateMachineInput{Name: &name})
}

func (p *Poller) UpdateMachine(ctx context.Context, accountID, machineID string, in instabox.UpdateMachineInput) (*store.MachineRecord, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if _, err := p.store.GetMachine(ctx, accountID, machineID); err != nil {
		return nil, err
	}
	updated, err := p.client.UpdateMachine(ctx, apiKey, machineID, in)
	if err != nil {
		return nil, err
	}
	if err := p.syncSingleMachine(ctx, accountID, *updated); err != nil {
		return nil, err
	}
	return p.store.GetMachine(ctx, accountID, machineID)
}

func (p *Poller) ForkMachine(ctx context.Context, accountID, machineID string, in instabox.ForkMachineInput) (*instabox.ActionResult, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if _, err := p.store.GetMachine(ctx, accountID, machineID); err != nil {
		return nil, err
	}
	res, err := p.client.ForkMachine(ctx, apiKey, machineID, in)
	if err != nil {
		return nil, err
	}
	if res.Operation != nil && res.Operation.ID != "" {
		if res.Operation.Machine != nil {
			_ = p.syncSingleMachine(ctx, accountID, *res.Operation.Machine)
		}
		if res.Operation.State != "succeeded" && res.Operation.State != "failed" {
			p.trackOperation(accountID, apiKey, res.Operation.ID)
		}
	}
	return res, nil
}

func (p *Poller) DeleteMachine(ctx context.Context, accountID, machineID string) (*instabox.ActionResult, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	m, err := p.store.GetMachine(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	usageKey := accountID + ":" + machineID
	p.cancelKeepaliveCycle(ctx, usageKey, 2*time.Second)

	res, err := p.client.DeleteMachine(ctx, apiKey, machineID, m.MachineID)
	if err != nil {
		return nil, err
	}

	if res.Operation != nil && res.Operation.State == "succeeded" {
		unlock := p.lockAccount(accountID)
		_ = p.store.DeleteMachine(ctx, accountID, machineID)
		unlock()
		p.lastUsageCheck.Delete(usageKey)
		p.inactiveMetricDone.Delete(usageKey)
		p.keepaliveLeaseRunStart.Delete(usageKey)
		p.keepaliveCtrls.Delete(usageKey)
		return res, nil
	}

	syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = p.SyncAccount(syncCtx, accountID)
	if res.Operation != nil && res.Operation.ID != "" && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
		p.trackOperation(accountID, apiKey, res.Operation.ID)
	}
	return res, nil
}

func (p *Poller) CreateDeleteChallenge(ctx context.Context, accountID, machineID string) (*DeleteConfirmationChallenge, error) {
	acc, err := p.store.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	m, err := p.store.GetMachine(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	raw, _, err := crypto.RandomToken(18)
	if err != nil {
		return nil, err
	}
	token := "del_" + raw
	now := time.Now()
	const ttl = 5 * time.Minute

	p.deleteChallengeMu.Lock()
	if p.deleteChallenges == nil {
		p.deleteChallenges = make(map[string]deleteChallengeEntry)
	}
	for k, v := range p.deleteChallenges {
		if now.After(v.expiresAt) {
			delete(p.deleteChallenges, k)
		}
	}
	if len(p.deleteChallenges) >= 256 {
		for k := range p.deleteChallenges {
			delete(p.deleteChallenges, k)
			if len(p.deleteChallenges) < 128 {
				break
			}
		}
	}
	p.deleteChallenges[token] = deleteChallengeEntry{
		accountID:   accountID,
		machineID:   machineID,
		machineName: m.Name,
		expiresAt:   now.Add(ttl),
	}
	p.deleteChallengeMu.Unlock()

	return &DeleteConfirmationChallenge{
		RequiresUserConfirmation: true,
		ConfirmationToken:        token,
		AccountID:                accountID,
		AccountAlias:             acc.Alias,
		MachineID:                machineID,
		MachineName:              m.Name,
		ExpiresInSeconds:         int(ttl.Seconds()),
		Instruction: fmt.Sprintf(
			"STOP and ask the user to confirm permanently deleting machine %q (%s) on account %q (%s). Call again with confirmationToken ONLY after the user explicitly approves.",
			m.Name, machineID, acc.Alias, accountID,
		),
	}, nil
}

func (p *Poller) ConsumeDeleteChallenge(accountID, machineID, token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ErrDeleteConfirmationInvalid
	}
	now := time.Now()
	p.deleteChallengeMu.Lock()
	defer p.deleteChallengeMu.Unlock()
	if p.deleteChallenges == nil {
		return ErrDeleteConfirmationInvalid
	}
	entry, ok := p.deleteChallenges[trimmed]
	if !ok {
		return ErrDeleteConfirmationInvalid
	}
	if now.After(entry.expiresAt) {
		delete(p.deleteChallenges, trimmed)
		return ErrDeleteConfirmationInvalid
	}
	if entry.accountID != accountID || entry.machineID != machineID {
		return ErrDeleteConfirmationInvalid
	}
	delete(p.deleteChallenges, trimmed)
	return nil
}

func (p *Poller) resolveMachineKey(ctx context.Context, accountID, machineID string) (string, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return "", err
	}
	if _, err := p.store.GetMachine(ctx, accountID, machineID); err != nil {
		return "", err
	}
	return apiKey, nil
}

func (p *Poller) CheckMachineExists(ctx context.Context, accountID, machineID string) error {
	_, err := p.resolveMachineKey(ctx, accountID, machineID)
	return err
}

func (p *Poller) DialTerminal(ctx context.Context, accountID, machineID string, opts instabox.TerminalOptions) (*ws.Conn, *instabox.APIError, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, nil, err
	}
	return p.client.DialTerminal(ctx, apiKey, machineID, opts)
}

func (p *Poller) ExecMachine(ctx context.Context, accountID, machineID string, in instabox.ExecInput) (*instabox.ExecResult, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.ExecMachine(ctx, apiKey, machineID, in)
}

func (p *Poller) ReadMachineFiles(ctx context.Context, accountID, machineID, filePath string) (*instabox.MachineFiles, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.ReadMachineFiles(ctx, apiKey, machineID, filePath)
}

func (p *Poller) WriteMachineFile(ctx context.Context, accountID, machineID, filePath, mode string, parents bool, content []byte) (*instabox.FileWritten, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.WriteMachineFile(ctx, apiKey, machineID, filePath, mode, parents, content)
}

func (p *Poller) ListMachinePorts(ctx context.Context, accountID, machineID string) (*instabox.PublicPortList, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.ListMachinePorts(ctx, apiKey, machineID)
}

func (p *Poller) PublishMachinePort(ctx context.Context, accountID, machineID string, in instabox.PublishPortInput) (*instabox.PublicPort, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.PublishMachinePort(ctx, apiKey, machineID, in)
}

func (p *Poller) UnpublishMachinePort(ctx context.Context, accountID, machineID string, port int) error {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return err
	}
	return p.client.UnpublishMachinePort(ctx, apiKey, machineID, port)
}

func (p *Poller) ListMachineProcesses(ctx context.Context, accountID, machineID string, all bool) (*instabox.ProcessList, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.ListMachineProcesses(ctx, apiKey, machineID, all)
}

func (p *Poller) GetMachineLogs(ctx context.Context, accountID, machineID string, tail int, after int64) (*instabox.MachineLogs, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.GetMachineLogs(ctx, apiKey, machineID, tail, after)
}

func (p *Poller) GetMachineConnectionInfo(ctx context.Context, accountID, machineID string) (*instabox.ConnectionInfo, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.GetMachineConnectionInfo(ctx, apiKey, machineID)
}

func (p *Poller) ListSnapshots(ctx context.Context, accountID string) ([]instabox.Snapshot, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return p.client.ListSnapshots(ctx, apiKey)
}

func (p *Poller) CreateSnapshot(ctx context.Context, accountID, machineID string, in instabox.CreateSnapshotInput) (*instabox.Snapshot, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	return p.client.CreateSnapshot(ctx, apiKey, machineID, in)
}

func (p *Poller) RestoreMachine(ctx context.Context, accountID, machineID string, in instabox.RestoreMachineInput) (*instabox.ActionResult, error) {
	apiKey, err := p.resolveMachineKey(ctx, accountID, machineID)
	if err != nil {
		return nil, err
	}
	res, err := p.client.RestoreMachine(ctx, apiKey, machineID, in)
	if err != nil {
		return nil, err
	}
	if res.Operation != nil && res.Operation.ID != "" && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
		p.trackOperation(accountID, apiKey, res.Operation.ID)
	} else {
		syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		_ = p.SyncAccount(syncCtx, accountID)
	}
	return res, nil
}

func (p *Poller) DeleteSnapshot(ctx context.Context, accountID, snapshotID, confirmValue string) error {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return err
	}
	return p.client.DeleteSnapshot(ctx, apiKey, snapshotID, confirmValue)
}

func (p *Poller) GetOperation(ctx context.Context, accountID, opID, wait string) (*instabox.Operation, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	op, err := p.client.GetOperation(ctx, apiKey, opID, wait)
	if err != nil {
		return nil, err
	}
	if op != nil && op.Machine != nil {
		_ = p.syncSingleMachine(ctx, accountID, *op.Machine)
	}
	return op, nil
}

func (p *Poller) trackOperation(accountID, apiKey, opID string) {
	if p.ctx.Err() != nil {
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ctx, cancel := context.WithTimeout(p.ctx, 75*time.Second)
		defer cancel()
		for range 6 {
			op, err := p.client.GetOperation(ctx, apiKey, opID, "12s")
			if err != nil {
				return
			}
			if op.State != "queued" && op.State != "running" {
				_ = p.SyncAccount(ctx, accountID)
				return
			}
		}
		_ = p.SyncAccount(ctx, accountID)
	}()
}

func toMachineRecord(accountID string, m instabox.Machine) store.MachineRecord {
	raw, _ := json.Marshal(m)
	rec := store.MachineRecord{
		AccountID:               accountID,
		MachineID:               m.ID,
		Name:                    m.Name,
		State:                   m.State,
		DesiredState:            m.DesiredState,
		VCPUs:                   m.Spec.VCPUs,
		MemMiB:                  m.Spec.MemMiB,
		PersistentDiskGiB:       m.Spec.PersistentDiskGiB,
		RootDiskGiB:             m.Spec.RootDiskGiB,
		TemplateID:              m.Template.ID,
		Free:                    m.Free,
		Ready:                   m.Runtime.Ready,
		ProcessesPreserved:      m.Runtime.ProcessesPreserved,
		AutoSuspendAfterSeconds: m.AutoSuspendAfterSeconds,
		MaxRunMinutes:           m.MaxRunMinutes,
		RunStartedAt:            m.RunStartedAt,
		AutoSuspendedAt:         m.AutoSuspendedAt,
		CooldownUntil:           m.CooldownUntil,
		CooldownReason:          m.CooldownReason,
		ArchiveAt:               m.ArchiveAt,
		DeleteAt:                m.DeleteAt,
		OperationID:             m.OperationID,
		RawJSON:                 raw,
	}
	if m.LastError != nil {
		rec.LastErrorCode = m.LastError.Code
		rec.LastErrorMsg = m.LastError.Message
	}
	return rec
}

func (p *Poller) SetKeepalivePollDelay(d time.Duration) {
	if d <= 0 {
		p.keepaliveDelayOverride.Store(0)
		return
	}
	p.keepaliveDelayOverride.Store(d.Nanoseconds())
}

func (p *Poller) keepalivePollDelay(defaultDelay time.Duration) time.Duration {
	if ov := p.keepaliveDelayOverride.Load(); ov > 0 {
		return time.Duration(ov)
	}
	return defaultDelay
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func parseInstaboxTime(raw string) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

func keepaliveRestartThresholdSec(m instabox.Machine) int64 {
	if m.MaxRunMinutes <= 5 {
		return 0
	}
	// Free runs start at 120m and extend at T-5m while active; checking at T-3m
	// avoids interrupting an active extension while preventing cooldown lockouts,
	// and caps at 25200s (7h) ahead of the 8h (480m) limit as in render_watch.py.
	threshold := int64(m.MaxRunMinutes)*60 - 180
	if threshold > keepaliveRestartCapMaxSec {
		threshold = keepaliveRestartCapMaxSec
	}
	return threshold
}

func isTransitionalState(state string) bool {
	switch state {
	case "starting", "resuming", "stopping", "pausing", "suspending", "creating", "deleting":
		return true
	default:
		return false
	}
}

func (p *Poller) upstreamNow() time.Time {
	return time.Now().UTC().Add(time.Duration(p.clockOffsetMs.Load()) * time.Millisecond)
}

func isCooldownActive(m instabox.Machine, upNow time.Time) bool {
	cd, ok := parseInstaboxTime(m.CooldownUntil)
	return ok && cd.After(upNow)
}

func (p *Poller) needsKeepaliveAction(m instabox.Machine, rec store.MachineRecord) bool {
	if !rec.KeepaliveEnabled || isTransitionalState(m.State) {
		return false
	}
	upNow := p.upstreamNow()
	upUnix := upNow.Unix()
	if m.State == "running" {
		threshold := keepaliveRestartThresholdSec(m)
		if threshold > 0 {
			if rs, ok := parseInstaboxTime(m.RunStartedAt); ok && int64(upNow.Sub(rs).Seconds()) >= threshold {
				return true
			}
		}
		key := rec.AccountID + ":" + m.ID
		prevRS, hasRS := p.keepaliveLeaseRunStart.Load(key)
		if m.RunStartedAt != "" && (!hasRS || prevRS.(string) != m.RunStartedAt) {
			return true
		}
		return rec.KeepaliveLeaseExpiresAt <= upUnix+keepaliveRenewLeadSeconds
	}
	if m.State == "suspended" && m.AutoSuspendedAt != "" {
		if isCooldownActive(m, upNow) {
			return false
		}
		return true
	}
	if m.State == "stopped" && rec.KeepalivePendingRestart {
		if isCooldownActive(m, upNow) {
			return false
		}
		return true
	}
	return false
}

func (p *Poller) reconcileSkippedKeepalive(ctx context.Context, accountID string, m instabox.Machine, prev store.MachineRecord) {
	key := accountID + ":" + m.ID
	if p.isKeepaliveInFlight(key) || isTransitionalState(m.State) {
		return
	}
	if m.State == "running" {
		if prev.KeepalivePendingRestart {
			_ = p.store.SetMachineKeepalivePendingRestart(ctx, accountID, m.ID, false)
		}
		return
	}
	upNow := p.upstreamNow()
	var targetAction string
	targetErr := ""
	switch {
	case isCooldownActive(m, upNow) && ((m.State == "suspended" && m.AutoSuspendedAt != "") || (m.State == "stopped" && prev.KeepalivePendingRestart)):
		targetAction = "cooldown_wait"
	case (m.State == "stopped" && !prev.KeepalivePendingRestart) || (m.State == "suspended" && m.AutoSuspendedAt == ""):
		targetAction = "manual_paused"
	case m.State == "error":
		if prev.KeepaliveLeaseExpiresAt == 0 {
			return
		}
		targetAction = "error"
		targetErr = prev.KeepaliveLastError
		if m.LastError != nil && m.LastError.Message != "" {
			targetErr = m.LastError.Message
		}
	default:
		return
	}
	ctrl := p.getKeepaliveCtrl(key)
	ctrl.mu.Lock()
	if ctrl.inFlight {
		ctrl.mu.Unlock()
		return
	}
	p.keepaliveLeaseRunStart.Delete(key)
	ctrl.mu.Unlock()

	if prev.KeepaliveLeaseExpiresAt != 0 || prev.KeepaliveLastAction != targetAction || prev.KeepaliveLastError != targetErr {
		_ = p.store.UpdateMachineKeepaliveState(ctx, accountID, m.ID, 0, time.Now().Unix(), targetAction, targetErr)
	}
}

func (p *Poller) getKeepaliveCtrl(key string) *keepaliveCtrl {
	if v, ok := p.keepaliveCtrls.Load(key); ok {
		return v.(*keepaliveCtrl)
	}
	c := &keepaliveCtrl{}
	actual, _ := p.keepaliveCtrls.LoadOrStore(key, c)
	return actual.(*keepaliveCtrl)
}

func (p *Poller) isKeepaliveInFlight(key string) bool {
	ctrl := p.getKeepaliveCtrl(key)
	ctrl.mu.Lock()
	inFlight := ctrl.inFlight
	ctrl.mu.Unlock()
	return inFlight
}

func (p *Poller) keepaliveValid(ctx context.Context, key string, gen uint64) bool {
	if ctx.Err() != nil {
		return false
	}
	ctrl := p.getKeepaliveCtrl(key)
	ctrl.mu.Lock()
	ok := ctrl.gen == gen && !ctrl.cancelled
	ctrl.mu.Unlock()
	return ok
}

func (p *Poller) storeKeepaliveLeaseRunStart(ctx context.Context, key string, gen uint64, runStart string) bool {
	if ctx.Err() != nil {
		return false
	}
	ctrl := p.getKeepaliveCtrl(key)
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	if ctrl.gen != gen || ctrl.cancelled {
		return false
	}
	p.keepaliveLeaseRunStart.Store(key, runStart)
	return true
}

func (p *Poller) deleteKeepaliveLeaseRunStart(key string) {
	ctrl := p.getKeepaliveCtrl(key)
	ctrl.mu.Lock()
	p.keepaliveLeaseRunStart.Delete(key)
	ctrl.mu.Unlock()
}

func (p *Poller) updateKeepaliveState(ctx context.Context, accountID, machineID string, gen uint64, leaseExpiresAt, lastAt int64, lastAction, lastErr string) {
	key := accountID + ":" + machineID
	if !p.keepaliveValid(ctx, key, gen) {
		return
	}
	_ = p.store.UpdateMachineKeepaliveState(ctx, accountID, machineID, leaseExpiresAt, lastAt, lastAction, lastErr)
}

func (p *Poller) cancelKeepaliveCycle(ctx context.Context, key string, waitTimeout time.Duration) {
	ctrl := p.getKeepaliveCtrl(key)
	ctrl.mu.Lock()
	ctrl.gen++
	var cancel context.CancelFunc
	var done chan struct{}
	if ctrl.inFlight {
		ctrl.cancelled = true
		cancel = ctrl.cancel
		done = ctrl.done
	}
	ctrl.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		if waitTimeout > 0 {
			timer := time.NewTimer(waitTimeout)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
			case <-ctx.Done():
			}
		} else {
			select {
			case <-done:
			case <-ctx.Done():
			}
		}
	}
}

func (p *Poller) SetMachineKeepalive(ctx context.Context, accountID, machineID string, enabled bool) (*store.MachineRecord, error) {
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if _, err := p.store.GetMachine(ctx, accountID, machineID); err != nil {
		return nil, err
	}
	key := accountID + ":" + machineID
	ctrl := p.getKeepaliveCtrl(key)
	ctrl.toggleMu.Lock()
	defer ctrl.toggleMu.Unlock()

	if err := p.store.SetMachineKeepalive(ctx, accountID, machineID, enabled); err != nil {
		return nil, err
	}
	if enabled {
		ctrl.mu.Lock()
		ctrl.gen++
		ctrl.cancelled = false
		ctrl.mu.Unlock()
		p.triggerKeepalive(accountID, machineID, true)
	} else {
		p.cancelKeepaliveCycle(ctx, key, 3*time.Second)
		p.deleteKeepaliveLeaseRunStart(key)
		delCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = p.client.DeleteActivityLease(delCtx, apiKey, machineID, keepaliveLeaseID)
	}
	return p.store.GetMachine(ctx, accountID, machineID)
}

func (p *Poller) triggerKeepalive(accountID, machineID string, forceLease bool) {
	if p.ctx.Err() != nil {
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ctx, cancel := context.WithTimeout(p.ctx, 120*time.Second)
		defer cancel()
		if err := p.RunKeepaliveCycle(ctx, accountID, machineID, forceLease); err != nil && p.ctx.Err() == nil && ctx.Err() == nil {
			log.Printf("keepalive %s/%s: %v", accountID, machineID, err)
		}
	}()
}

func (p *Poller) RunKeepaliveCycle(ctx context.Context, accountID, machineID string, forceLease bool) error {
	key := accountID + ":" + machineID
	ctrl := p.getKeepaliveCtrl(key)

	var myGen uint64
	var cycleCtx context.Context
	var cancel context.CancelFunc
	var done chan struct{}
	for {
		ctrl.mu.Lock()
		if ctrl.inFlight {
			if !forceLease && !ctrl.cancelled {
				ctrl.mu.Unlock()
				return nil
			}
			oldDone := ctrl.done
			ctrl.mu.Unlock()
			select {
			case <-oldDone:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		myGen = ctrl.gen
		cycleCtx, cancel = context.WithCancel(ctx)
		done = make(chan struct{})
		ctrl.inFlight = true
		ctrl.cancelled = false
		ctrl.cancel = cancel
		ctrl.done = done
		ctrl.mu.Unlock()
		break
	}
	defer func() {
		cancel()
		ctrl.mu.Lock()
		ctrl.inFlight = false
		ctrl.cancelled = false
		ctrl.cancel = nil
		ctrl.done = nil
		ctrl.mu.Unlock()
		close(done)
	}()

	rec, err := p.store.GetMachine(cycleCtx, accountID, machineID)
	if err != nil {
		return err
	}
	if !rec.KeepaliveEnabled || !p.keepaliveValid(cycleCtx, key, myGen) {
		return nil
	}
	apiKey, err := p.store.GetAccountAPIKey(cycleCtx, accountID)
	if err != nil {
		return err
	}

	m, err := p.client.GetMachine(cycleCtx, apiKey, machineID)
	if err != nil {
		p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, -1, time.Now().Unix(), "error", err.Error())
		return err
	}
	if !p.keepaliveValid(cycleCtx, key, myGen) {
		return nil
	}
	rec, err = p.store.GetMachine(cycleCtx, accountID, machineID)
	if err != nil || !rec.KeepaliveEnabled || !p.keepaliveValid(cycleCtx, key, myGen) {
		return nil
	}
	if err := p.syncSingleMachine(cycleCtx, accountID, *m); err != nil {
		if !p.keepaliveValid(cycleCtx, key, myGen) {
			return nil
		}
	}
	if !p.keepaliveValid(cycleCtx, key, myGen) {
		return nil
	}

	if isTransitionalState(m.State) {
		p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, -1, time.Now().Unix(), "transition", "")
		return nil
	}

	nowUnix := time.Now().Unix()
	upNow := p.upstreamNow()
	upUnix := upNow.Unix()

	if m.State == "running" {
		threshold := keepaliveRestartThresholdSec(*m)
		if threshold > 0 {
			if rs, ok := parseInstaboxTime(m.RunStartedAt); ok {
				rt := int64(upNow.Sub(rs).Seconds())
				if rt >= threshold {
					return p.powerCycleForKeepalive(cycleCtx, accountID, apiKey, machineID, key, myGen)
				}
			}
		}
		if rec.KeepalivePendingRestart {
			_ = p.store.SetMachineKeepalivePendingRestart(cycleCtx, accountID, machineID, false)
		}

		// instabox.sh Nov-2026 free-plan runtime rules: activity leases no longer
		// extend the run — only real use does (an exec, file access, port traffic,
		// desktop/SSH input, or >=0.25 vCPU load). So instead of renewing a lease
		// we run a trivial command in the machine: a single exec counts as use for
		// its minute, which within the last 30m of a run buys the +30m extension
		// (and shows as light CPU on the machine's own metrics). The lease is kept
		// as a secondary guard against the 30-minute idle pause only.
		prevRS, hasRS := p.keepaliveLeaseRunStart.Load(key)
		runChanged := m.RunStartedAt != "" && (!hasRS || prevRS.(string) != m.RunStartedAt)
		expiring := rec.KeepaliveLeaseExpiresAt <= upUnix+keepaliveRenewLeadSeconds
		if !forceLease && !runChanged && !expiring {
			return nil
		}
		if !p.keepaliveValid(cycleCtx, key, myGen) {
			return nil
		}

		expUnix := p.upstreamNow().Unix() + keepaliveLeaseTTLSeconds
		if lease, err := p.client.PutActivityLease(cycleCtx, apiKey, machineID, keepaliveLeaseID, keepaliveLeaseTTLSeconds); err == nil && lease != nil {
			if expTime, ok := parseInstaboxTime(lease.ExpiresAt); ok {
				expUnix = expTime.Unix()
			}
		}

		execRes, execErr := p.client.ExecMachine(cycleCtx, apiKey, machineID, instabox.ExecInput{
			Cmd:            "echo ibmon-activity $(date +%s) >/dev/null 2>&1; true",
			TimeoutSeconds: 20,
		})
		if execErr != nil {
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, -1, time.Now().Unix(), "error", execErr.Error())
			return execErr
		}
		if !p.keepaliveValid(cycleCtx, key, myGen) {
			return nil
		}
		if execRes != nil && execRes.ExitCode != 0 {
			msg := fmt.Sprintf("activity exec exit %d", execRes.ExitCode)
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, -1, time.Now().Unix(), "error", msg)
			return errors.New(msg)
		}
		if !p.storeKeepaliveLeaseRunStart(cycleCtx, key, myGen, m.RunStartedAt) {
			return nil
		}
		p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, expUnix, time.Now().Unix(), "activity_ping", "")
		return nil
	}

	if m.State == "stopped" && rec.KeepalivePendingRestart {
		if isCooldownActive(*m, upNow) {
			p.deleteKeepaliveLeaseRunStart(key)
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, nowUnix, "cooldown_wait", "")
			return nil
		}
		return p.startForKeepalive(cycleCtx, accountID, apiKey, machineID, key, myGen, p.keepalivePollDelay(2*time.Second))
	}

	if m.State == "suspended" && m.AutoSuspendedAt != "" {
		if isCooldownActive(*m, upNow) {
			p.deleteKeepaliveLeaseRunStart(key)
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, nowUnix, "cooldown_wait", "")
			return nil
		}
		if !p.keepaliveValid(cycleCtx, key, myGen) {
			return nil
		}

		wakeStep := p.keepalivePollDelay(3 * time.Second)
		if _, err := p.client.PerformAction(cycleCtx, apiKey, machineID, instabox.ActionWake); err != nil {
			if !p.keepaliveValid(cycleCtx, key, myGen) {
				return nil
			}
			var apiErr *instabox.APIError
			if errors.As(err, &apiErr) && apiErr.Code == "machine_cooldown" {
				p.deleteKeepaliveLeaseRunStart(key)
				p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, nowUnix, "cooldown_wait", "")
				return nil
			}
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, nowUnix, "error", err.Error())
			return err
		}

		var cur *instabox.Machine
		for range 8 {
			if !sleepWithContext(cycleCtx, wakeStep) {
				if !p.keepaliveValid(cycleCtx, key, myGen) {
					return nil
				}
				return cycleCtx.Err()
			}
			if !p.keepaliveValid(cycleCtx, key, myGen) {
				return nil
			}
			got, gErr := p.client.GetMachine(cycleCtx, apiKey, machineID)
			if gErr == nil && got != nil {
				cur = got
				if got.State == "running" ||
					got.State == "stopped" ||
					(got.State == "suspended" && got.AutoSuspendedAt == "") ||
					isCooldownActive(*got, p.upstreamNow()) {
					break
				}
			}
		}
		if !p.keepaliveValid(cycleCtx, key, myGen) {
			return nil
		}

		if cur != nil {
			if cur.State == "stopped" || (cur.State == "suspended" && cur.AutoSuspendedAt == "") {
				_ = p.syncSingleMachine(cycleCtx, accountID, *cur)
				p.deleteKeepaliveLeaseRunStart(key)
				p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, time.Now().Unix(), "manual_paused", "")
				return nil
			}
			if isCooldownActive(*cur, p.upstreamNow()) {
				_ = p.syncSingleMachine(cycleCtx, accountID, *cur)
				p.deleteKeepaliveLeaseRunStart(key)
				p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, time.Now().Unix(), "cooldown_wait", "")
				return nil
			}
		}

		usedAction := "wake"
		if cur == nil || cur.State != "running" {
			usedAction = "resume"
			if _, err := p.client.PerformAction(cycleCtx, apiKey, machineID, instabox.ActionResume); err != nil {
				if !p.keepaliveValid(cycleCtx, key, myGen) {
					return nil
				}
				var apiErr *instabox.APIError
				if errors.As(err, &apiErr) && apiErr.Code == "machine_cooldown" {
					p.deleteKeepaliveLeaseRunStart(key)
					p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, time.Now().Unix(), "cooldown_wait", "")
					return nil
				}
				p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, time.Now().Unix(), "error", err.Error())
				return err
			}
			for range 6 {
				if !sleepWithContext(cycleCtx, wakeStep) {
					if !p.keepaliveValid(cycleCtx, key, myGen) {
						return nil
					}
					return cycleCtx.Err()
				}
				if !p.keepaliveValid(cycleCtx, key, myGen) {
					return nil
				}
				got, gErr := p.client.GetMachine(cycleCtx, apiKey, machineID)
				if gErr == nil && got != nil {
					cur = got
					if got.State == "running" ||
						got.State == "stopped" ||
						(got.State == "suspended" && got.AutoSuspendedAt == "") ||
						isCooldownActive(*got, p.upstreamNow()) {
						break
					}
				}
			}
		}
		if !p.keepaliveValid(cycleCtx, key, myGen) {
			return nil
		}

		if cur != nil {
			_ = p.syncSingleMachine(cycleCtx, accountID, *cur)
		}
		if !p.keepaliveValid(cycleCtx, key, myGen) {
			return nil
		}
		if cur != nil && cur.State == "running" {
			if rec.KeepalivePendingRestart {
				_ = p.store.SetMachineKeepalivePendingRestart(cycleCtx, accountID, machineID, false)
			}
			var expUnix int64
			if lease, lErr := p.client.PutActivityLease(cycleCtx, apiKey, machineID, keepaliveLeaseID, keepaliveLeaseTTLSeconds); lErr == nil && lease != nil {
				if !p.keepaliveValid(cycleCtx, key, myGen) {
					return nil
				}
				expUnix = p.upstreamNow().Unix() + keepaliveLeaseTTLSeconds
				if expTime, ok := parseInstaboxTime(lease.ExpiresAt); ok {
					expUnix = expTime.Unix()
				}
				if !p.storeKeepaliveLeaseRunStart(cycleCtx, key, myGen, cur.RunStartedAt) {
					return nil
				}
			}
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, expUnix, time.Now().Unix(), usedAction, "")
		} else if cur != nil && (cur.State == "stopped" || (cur.State == "suspended" && cur.AutoSuspendedAt == "")) {
			p.deleteKeepaliveLeaseRunStart(key)
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, time.Now().Unix(), "manual_paused", "")
		} else if cur != nil && isCooldownActive(*cur, p.upstreamNow()) {
			p.deleteKeepaliveLeaseRunStart(key)
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, time.Now().Unix(), "cooldown_wait", "")
		} else {
			p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, time.Now().Unix(), usedAction, "")
		}
		return nil
	}

	p.deleteKeepaliveLeaseRunStart(key)
	p.updateKeepaliveState(cycleCtx, accountID, machineID, myGen, 0, nowUnix, "manual_paused", "")
	return nil
}

func (p *Poller) powerCycleForKeepalive(ctx context.Context, accountID, apiKey, machineID, key string, gen uint64) error {
	step := p.keepalivePollDelay(2 * time.Second)
	if !p.keepaliveValid(ctx, key, gen) {
		return nil
	}
	if err := p.store.SetMachineKeepalivePendingRestart(ctx, accountID, machineID, true); err != nil {
		return err
	}
	if _, err := p.client.PerformAction(ctx, apiKey, machineID, instabox.ActionStop); err != nil {
		if !p.keepaliveValid(ctx, key, gen) {
			return nil
		}
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			_ = p.store.SetMachineKeepalivePendingRestart(ctx, accountID, machineID, false)
		}
		p.updateKeepaliveState(ctx, accountID, machineID, gen, 0, time.Now().Unix(), "error", err.Error())
		return err
	}
	stopped := false
	for range 20 {
		if !sleepWithContext(ctx, step) {
			if !p.keepaliveValid(ctx, key, gen) {
				return nil
			}
			return ctx.Err()
		}
		if !p.keepaliveValid(ctx, key, gen) {
			return nil
		}
		cur, err := p.client.GetMachine(ctx, apiKey, machineID)
		if err == nil && cur != nil {
			if !p.keepaliveValid(ctx, key, gen) {
				return nil
			}
			_ = p.syncSingleMachine(ctx, accountID, *cur)
			if cur.State == "stopped" {
				stopped = true
				break
			}
		}
	}
	if !p.keepaliveValid(ctx, key, gen) {
		return nil
	}
	if !stopped {
		err := errors.New("machine did not stop before power-cycle timeout")
		p.updateKeepaliveState(ctx, accountID, machineID, gen, 0, time.Now().Unix(), "error", err.Error())
		return err
	}

	return p.startForKeepalive(ctx, accountID, apiKey, machineID, key, gen, step)
}

func (p *Poller) startForKeepalive(ctx context.Context, accountID, apiKey, machineID, key string, gen uint64, step time.Duration) error {
	if !p.keepaliveValid(ctx, key, gen) {
		return nil
	}
	if _, err := p.client.PerformAction(ctx, apiKey, machineID, instabox.ActionStart); err != nil {
		if !p.keepaliveValid(ctx, key, gen) {
			return nil
		}
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "machine_cooldown" {
			p.updateKeepaliveState(ctx, accountID, machineID, gen, 0, time.Now().Unix(), "cooldown_wait", "")
			return nil
		}
		p.updateKeepaliveState(ctx, accountID, machineID, gen, 0, time.Now().Unix(), "error", err.Error())
		return err
	}
	var runningMachine *instabox.Machine
	for range 25 {
		if !sleepWithContext(ctx, step) {
			if !p.keepaliveValid(ctx, key, gen) {
				return nil
			}
			return ctx.Err()
		}
		if !p.keepaliveValid(ctx, key, gen) {
			return nil
		}
		cur, err := p.client.GetMachine(ctx, apiKey, machineID)
		if err == nil && cur != nil {
			if !p.keepaliveValid(ctx, key, gen) {
				return nil
			}
			_ = p.syncSingleMachine(ctx, accountID, *cur)
			if cur.State == "running" {
				runningMachine = cur
				break
			}
		}
	}
	if !p.keepaliveValid(ctx, key, gen) {
		return nil
	}
	if runningMachine == nil {
		err := errors.New("machine did not reach running state after power-cycle start")
		p.updateKeepaliveState(ctx, accountID, machineID, gen, 0, time.Now().Unix(), "error", err.Error())
		return err
	}

	_ = p.store.SetMachineKeepalivePendingRestart(ctx, accountID, machineID, false)

	var expUnix int64
	if lease, lErr := p.client.PutActivityLease(ctx, apiKey, machineID, keepaliveLeaseID, keepaliveLeaseTTLSeconds); lErr == nil && lease != nil {
		if !p.keepaliveValid(ctx, key, gen) {
			return nil
		}
		expUnix = p.upstreamNow().Unix() + keepaliveLeaseTTLSeconds
		if expTime, ok := parseInstaboxTime(lease.ExpiresAt); ok {
			expUnix = expTime.Unix()
		}
		if !p.storeKeepaliveLeaseRunStart(ctx, key, gen, runningMachine.RunStartedAt) {
			return nil
		}
	}
	p.updateKeepaliveState(ctx, accountID, machineID, gen, expUnix, time.Now().Unix(), "power_cycle", "")
	return nil
}
