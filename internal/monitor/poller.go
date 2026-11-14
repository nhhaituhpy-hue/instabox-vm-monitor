package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

type Poller struct {
	store         *store.Store
	client        *instabox.Client
	interval      time.Duration
	retentionDays int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	accountLocks       sync.Map // key: accountID -> *sync.Mutex
	clockOffsetMs      atomic.Int64
	lastPruneUnix      atomic.Int64
	lastUsageCheck     sync.Map // key: accountID+":"+machineID -> int64 unix
	inactiveMetricDone sync.Map // key: accountID+":"+machineID -> bool
}

func (p *Poller) lockAccount(accountID string) func() {
	muIface, _ := p.accountLocks.LoadOrStore(accountID, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
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

	res, err := p.client.PerformAction(ctx, apiKey, machineID, effective)
	if err != nil {
		return nil, err
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
	unlock := p.lockAccount(accountID)
	defer unlock()

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
	apiKey, err := p.store.GetAccountAPIKey(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if _, err := p.store.GetMachine(ctx, accountID, machineID); err != nil {
		return nil, err
	}
	updated, err := p.client.RenameMachine(ctx, apiKey, machineID, name)
	if err != nil {
		return nil, err
	}
	if err := p.syncSingleMachine(ctx, accountID, *updated); err != nil {
		return nil, err
	}
	return p.store.GetMachine(ctx, accountID, machineID)
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
	res, err := p.client.DeleteMachine(ctx, apiKey, machineID, m.MachineID)
	if err != nil {
		return nil, err
	}

	usageKey := accountID + ":" + machineID
	if res.Operation != nil && res.Operation.State == "succeeded" {
		unlock := p.lockAccount(accountID)
		_ = p.store.DeleteMachine(ctx, accountID, machineID)
		unlock()
		p.lastUsageCheck.Delete(usageKey)
		p.inactiveMetricDone.Delete(usageKey)
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
		OperationID:             m.OperationID,
		RawJSON:                 raw,
	}
	if m.LastError != nil {
		rec.LastErrorCode = m.LastError.Code
		rec.LastErrorMsg = m.LastError.Message
	}
	return rec
}
