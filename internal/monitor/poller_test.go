package monitor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/monitor"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

type rt struct {
	u *url.URL
	t http.RoundTripper
}

func (r *rt) RoundTrip(req *http.Request) (*http.Response, error) {
	c := req.Clone(req.Context())
	u := *c.URL
	u.Scheme = r.u.Scheme
	u.Host = r.u.Host
	c.URL = &u
	return r.t.RoundTrip(c)
}

func TestSyncAccountPopulatesMachinesDiskMetricsAndMonthlyUsage(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, err := store.Open(filepath.Join(dir, "mon.db"), key)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var diskCalls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch r.URL.Path {
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1,"limits":{"maxMachines":1}}`))
		case "/v1/machines":
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_run","name":"dev-box","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"runStartedAt":"2026-11-13T10:00:00Z","maxRunMinutes":480,"free":true}],"nextCursor":null}`))
		case "/v1/machines/m_run/usage":
			_, _ = w.Write([]byte(`{"from":"2026-11-01T00:00:00Z","to":"2026-11-13T12:00:00Z","runningSeconds":7200,"vcpuSeconds":28800,"memGiBSeconds":57600,"diskGiBSeconds":144000,"egressBytes":1024}`))
		case "/v1/machines/m_run/metrics":
			// Omit disk inside metrics to verify fallback to /disk for running machines
			ts := time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"metrics":[{"timestamp":"` + ts + `","cpuCount":4,"cpuUsedPct":27.5,"memUsed":3221225472,"memTotal":8589934592,"diskTotal":21474836480,"netInBytesPerSec":1200,"netOutBytesPerSec":600,"diskReadBytesPerSec":4096,"diskWriteBytesPerSec":8192}]}`))
		case "/v1/machines/m_run/disk":
			diskCalls.Add(1)
			_, _ = w.Write([]byte(`{"measuredAt":"2026-11-13T12:00:00Z","filesystems":[{"mount":"/data","sizeBytes":21474836480,"usedBytes":5368709120,"availableBytes":16106127360,"usedPct":25,"inodesUsedPct":3,"level":"ok"},{"mount":"/","sizeBytes":17179869184,"usedBytes":4294967296,"availableBytes":12884901888,"usedPct":25,"inodesUsedPct":5,"level":"ok"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, err := st.CreateAccount(context.Background(), store.AccountInput{
		Alias:  "Box 1",
		APIKey: "ibk_test_1234567890",
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	if err := p.SyncAccount(context.Background(), acc.ID); err != nil {
		t.Fatalf("SyncAccount: %v", err)
	}

	if diskCalls.Load() != 1 {
		t.Fatalf("expected 1 fallback /disk call, got %d", diskCalls.Load())
	}

	machines, err := st.ListMachines(context.Background())
	if err != nil || len(machines) != 1 {
		t.Fatalf("ListMachines got %d err=%v", len(machines), err)
	}
	m := machines[0]
	if m.MonthlyRunningSeconds != 7200 {
		t.Fatalf("expected monthlyRunningSeconds=7200, got %d", m.MonthlyRunningSeconds)
	}
	if m.LatestMetric == nil || m.LatestMetric.DiskUsed != 5368709120 || m.LatestMetric.RootUsed != 4294967296 {
		t.Fatalf("expected latest metric with /data and / disk bytes, got %+v", m.LatestMetric)
	}
}

func TestWakeActionChoosesWakeForIdleAndResumeForManualPause(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var mu sync.Mutex
	var actionPaths []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			mu.Lock()
			actionPaths = append(actionPaths, r.URL.Path)
			mu.Unlock()
		}
		switch r.URL.Path {
		case "/v1/machines/m_idle/wake":
			_, _ = w.Write([]byte(`{"id":"m_idle","name":"idle-box","state":"running","desiredState":"running"}`))
		case "/v1/machines/m_manual/resume":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_res","machineId":"m_manual","state":"queued"}`))
		case "/v1/machines/m_run/stop":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_stop","machineId":"m_run","state":"queued"}`))
		case "/v1/operations/op_res", "/v1/operations/op_stop":
			_, _ = w.Write([]byte(`{"id":"op_done","state":"succeeded"}`))
		default:
			_, _ = w.Write([]byte(`{"machines":[],"nextCursor":null}`))
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_abc123456789"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_idle", Name: "idle-box", State: "suspended", DesiredState: "suspended", AutoSuspendedAt: "2026-11-13T11:00:00Z"},
		{AccountID: acc.ID, MachineID: "m_manual", Name: "manual-box", State: "suspended", DesiredState: "suspended"},
		{AccountID: acc.ID, MachineID: "m_run", Name: "run-box", State: "running", DesiredState: "running"},
	})

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	defer p.Stop()
	if _, err := p.ExecuteAction(context.Background(), acc.ID, "m_idle", instabox.ActionWake); err != nil {
		t.Fatalf("wake idle err: %v", err)
	}
	if _, err := p.ExecuteAction(context.Background(), acc.ID, "m_manual", instabox.ActionWake); err != nil {
		t.Fatalf("wake manual err: %v", err)
	}
	if _, err := p.ExecuteAction(context.Background(), acc.ID, "m_run", instabox.ActionStop); err != nil {
		t.Fatalf("stop err: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), actionPaths...)
	mu.Unlock()
	if len(got) != 3 || got[0] != "/v1/machines/m_idle/wake" || got[1] != "/v1/machines/m_manual/resume" || got[2] != "/v1/machines/m_run/stop" {
		t.Fatalf("unexpected action endpoints called: %v", got)
	}
}

func TestStoppedVMCapturesFinalMetricsOnceThenSkipsRedundantPolling(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var metricsCalls atomic.Int32
	var stateMu sync.Mutex
	curState := "running"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		stateMu.Lock()
		stNow := curState
		stateMu.Unlock()
		switch r.URL.Path {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1}`))
		case "/v1/machines":
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_1","name":"vm1","state":"` + stNow + `","desiredState":"` + stNow + `","spec":{"vcpus":2,"memMiB":4096,"persistentDiskGiB":20},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"updatedAt":"2026-11-13T12:00:00Z"}],"nextCursor":null}`))
		case "/v1/machines/m_1/usage":
			_, _ = w.Write([]byte(`{"runningSeconds":100}`))
		case "/v1/machines/m_1/metrics":
			metricsCalls.Add(1)
			ts := time.Now().UTC().Add(-15 * time.Second).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"metrics":[{"timestamp":"` + ts + `","cpuCount":2,"cpuUsedPct":10.0,"memUsed":1000,"memTotal":4096,"diskTotal":20}]}`))
		case "/v1/machines/m_1/disk":
			_, _ = w.Write([]byte(`{"measuredAt":"2026-11-13T12:00:00Z","filesystems":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_abc123456789"})
	p := monitor.NewPoller(st, client, 30*time.Second, 7)

	// 1. Running cycle -> polls metrics (count=1)
	if err := p.SyncAccount(context.Background(), acc.ID); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	if metricsCalls.Load() != 1 {
		t.Fatalf("expected 1 metrics call while running, got %d", metricsCalls.Load())
	}

	// 2. Transitions to stopped (sample <120s old!) -> MUST still capture final metrics once (count=2)
	stateMu.Lock()
	curState = "stopped"
	stateMu.Unlock()
	if err := p.SyncAccount(context.Background(), acc.ID); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	if metricsCalls.Load() != 2 {
		t.Fatalf("expected 2nd metrics call on transition to stopped, got %d", metricsCalls.Load())
	}

	// 3. Subsequent cycles while still stopped -> MUST NOT poll /metrics again (stays 2)
	if err := p.SyncAccount(context.Background(), acc.ID); err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if err := p.SyncAccount(context.Background(), acc.ID); err != nil {
		t.Fatalf("sync 4: %v", err)
	}
	if metricsCalls.Load() != 2 {
		t.Fatalf("expected stopped VM to skip redundant metrics polls, got %d calls", metricsCalls.Load())
	}
}

func TestBackfillMetricsSinceNeverExceeds24Hours(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var capturedSince string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch r.URL.Path {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1}`))
		case "/v1/machines":
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_1","name":"vm1","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"updatedAt":"2026-11-13T12:00:00Z"}],"nextCursor":null}`))
		case "/v1/machines/m_1/usage":
			_, _ = w.Write([]byte(`{"runningSeconds":100}`))
		case "/v1/machines/m_1/metrics":
			capturedSince = r.URL.Query().Get("since")
			parsed, err := time.Parse(time.RFC3339, capturedSince)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// Enforce strict upstream < 24h check
			if time.Since(parsed) >= 24*time.Hour {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"code":"bad_request","message":"since must be within the last 24 hours"}}`))
				return
			}
			ts := time.Now().UTC().Add(-20 * time.Second).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"metrics":[{"timestamp":"` + ts + `","cpuCount":4,"cpuUsedPct":12.5,"memUsed":720000000,"memTotal":8589934592,"diskTotal":21474836480}]}`))
		case "/v1/machines/m_1/disk":
			_, _ = w.Write([]byte(`{"measuredAt":"2026-11-13T12:00:00Z","filesystems":[{"mount":"/data","sizeBytes":20957446144,"usedBytes":3373514752,"availableBytes":17567154176,"usedPct":16.2},{"mount":"/","sizeBytes":16729894912,"usedBytes":5021024256,"availableBytes":10833100800,"usedPct":31.7}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_abc123456789"})
	p := monitor.NewPoller(st, client, 30*time.Second, 7)

	if err := p.SyncAccount(context.Background(), acc.ID); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if capturedSince == "" {
		t.Fatalf("expected since query param to be captured")
	}
	samples, err := st.GetMachineMetrics(context.Background(), acc.ID, "m_1", 0)
	if err != nil || len(samples) != 1 || samples[0].CPUPct != 12.5 || samples[0].RootTotal == 0 {
		t.Fatalf("expected 1 metric sample with CPU & Root disk persisted, got err=%v samples=%+v", err, samples)
	}
}

func TestGetAccountMachineOptionsAndCreateMachine(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var created atomic.Bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"},"allowedSpecs":[{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/templates":
			_, _ = w.Write([]byte(`{"templates":[
				{"id":"base","version":"1","arch":"x86_64","hidden":true,"creatable":false,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20}},
				{"id":"agent-browser","version":"1","arch":"x86_64","creatable":true,"latest":false,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16}},
				{"id":"agent-browser","version":"2","description":"Default browser","arch":"x86_64","creatable":true,"latest":true,"default":true,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16}},
				{"id":"universal","version":"2","description":"Paid universal","arch":"x86_64","creatable":true,"latest":true,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16}},
				{"id":"macos","version":"1","arch":"arm64","os":"macos","creatable":true,"latest":true,"defaultSpec":{"vcpus":12,"memMiB":24576,"persistentDiskGiB":64}},
				{"id":"gpu","version":"1","arch":"x86_64","gpu":true,"creatable":true,"latest":true,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20}}
			]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines":
			created.Store(true)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_create_99","machineId":"m_new_99","type":"create","state":"succeeded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
			if created.Load() {
				_, _ = w.Write([]byte(`{"machines":[{"id":"m_new_99","name":"box-99","state":"running","desiredState":"running","template":{"id":"agent-browser"},"spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"maxRunMinutes":120,"free":true}],"nextCursor":null}`))
			} else {
				_, _ = w.Write([]byte(`{"machines":[],"nextCursor":null}`))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_new_99/usage":
			_, _ = w.Write([]byte(`{"runningSeconds":10}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_new_99/metrics":
			_, _ = w.Write([]byte(`{"metrics":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	p := monitor.NewPoller(st, client, 30*time.Second, 7)

	opts, err := p.GetAccountMachineOptions(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("GetAccountMachineOptions err = %v", err)
	}
	if opts.OrgTier != "free" || len(opts.AllowedSpecs) != 1 {
		t.Fatalf("unexpected opts: %+v", opts)
	}
	// Should deduplicate agent-browser (latest version 2), exclude hidden base, macos, gpu, and mark universal !PlanAllowed on free
	if len(opts.Templates) != 2 {
		t.Fatalf("expected 2 standard templates, got %d: %+v", len(opts.Templates), opts.Templates)
	}
	if opts.Templates[0].ID != "agent-browser" || !opts.Templates[0].PlanAllowed || !opts.Templates[0].Default {
		t.Fatalf("expected agent-browser default & allowed, got %+v", opts.Templates[0])
	}
	if opts.Templates[1].ID != "universal" || opts.Templates[1].PlanAllowed {
		t.Fatalf("expected universal !PlanAllowed on free tier, got %+v", opts.Templates[1])
	}

	res, err := p.CreateMachine(context.Background(), acc.ID, instabox.CreateMachineInput{Name: "box-99", TemplateID: "agent-browser"})
	if err != nil {
		t.Fatalf("CreateMachine err = %v", err)
	}
	if res.Operation == nil || res.Operation.MachineID != "m_new_99" {
		t.Fatalf("unexpected create op: %+v", res)
	}
	m, err := st.GetMachine(context.Background(), acc.ID, "m_new_99")
	if err != nil || m.Name != "box-99" {
		t.Fatalf("expected newly created machine to be synced into store, err=%v m=%+v", err, m)
	}
}

func TestRenameAndDeleteMachineUpdateLocalStore(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var deleted atomic.Bool
	var curName = "box-old"
	var mu sync.Mutex

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
			if deleted.Load() {
				_, _ = w.Write([]byte(`{"machines":[],"nextCursor":null}`))
				return
			}
			mu.Lock()
			nm := curName
			mu.Unlock()
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_target","name":"` + nm + `","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"free":true}],"nextCursor":null}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/machines/m_target":
			mu.Lock()
			curName = "box-renamed"
			mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"m_target","name":"box-renamed","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"free":true}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_target":
			if r.Header.Get("X-Confirm-Delete") == "" {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":{"code":"delete_confirmation_required","message":"missing X-Confirm-Delete"}}`))
				return
			}
			deleted.Store(true)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_del_99","machineId":"m_target","type":"delete","state":"succeeded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_target/usage":
			_, _ = w.Write([]byte(`{"runningSeconds":10}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_target/metrics":
			_, _ = w.Write([]byte(`{"metrics":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	p := monitor.NewPoller(st, client, 30*time.Second, 7)

	if err := p.SyncAccount(context.Background(), acc.ID); err != nil {
		t.Fatalf("initial sync err = %v", err)
	}

	renamed, err := p.RenameMachine(context.Background(), acc.ID, "m_target", "box-renamed")
	if err != nil || renamed.Name != "box-renamed" {
		t.Fatalf("RenameMachine failed: err=%v rec=%+v", err, renamed)
	}
	stored, err := st.GetMachine(context.Background(), acc.ID, "m_target")
	if err != nil || stored.Name != "box-renamed" {
		t.Fatalf("expected renamed machine in store, err=%v stored=%+v", err, stored)
	}

	delRes, err := p.DeleteMachine(context.Background(), acc.ID, "m_target")
	if err != nil || delRes.Operation == nil || delRes.Operation.ID != "op_del_99" {
		t.Fatalf("DeleteMachine failed: err=%v res=%+v", err, delRes)
	}
	if _, err := st.GetMachine(context.Background(), acc.ID, "m_target"); err == nil {
		t.Fatalf("expected deleted machine to be removed from store")
	}
}

func TestKeepaliveRenewsLeaseWakesIdlePowerCyclesCapAndPreservesManualStop(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	type vmState struct {
		state           string
		runStartedAt    string
		autoSuspendedAt string
		cooldownUntil   string
		maxRunMinutes   int
	}
	vms := map[string]*vmState{
		"m_lease": {
			state:         "running",
			runStartedAt:  time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339),
			maxRunMinutes: 480,
		},
		"m_cap": {
			state:         "running",
			runStartedAt:  time.Now().UTC().Add(-(7*time.Hour + 5*time.Minute)).Format(time.RFC3339),
			maxRunMinutes: 480,
		},
		"m_idle": {
			state:           "suspended",
			autoSuspendedAt: time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339),
			maxRunMinutes:   480,
		},
		"m_manual": {
			state:         "stopped",
			maxRunMinutes: 480,
		},
	}

	var mu sync.Mutex
	var events []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_lease":
			v := vms["m_lease"]
			_, _ = w.Write([]byte(`{"id":"m_lease","name":"vm-lease","state":"` + v.state + `","desiredState":"` + v.state + `","runStartedAt":"` + v.runStartedAt + `","maxRunMinutes":480,"free":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_lease/activity-leases/ibmon-keepalive":
			events = append(events, "put_lease:m_lease")
			exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_lease","expiresAt":"` + exp + `"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_lease/activity-leases/ibmon-keepalive":
			events = append(events, "delete_lease:m_lease")
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_cap":
			v := vms["m_cap"]
			_, _ = w.Write([]byte(`{"id":"m_cap","name":"vm-cap","state":"` + v.state + `","desiredState":"` + v.state + `","runStartedAt":"` + v.runStartedAt + `","maxRunMinutes":480,"free":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_cap/stop":
			events = append(events, "stop:m_cap")
			vms["m_cap"].state = "stopped"
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_stop","machineId":"m_cap","state":"succeeded"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_cap/start":
			events = append(events, "start:m_cap")
			vms["m_cap"].state = "running"
			vms["m_cap"].runStartedAt = time.Now().UTC().Format(time.RFC3339)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_start","machineId":"m_cap","state":"succeeded"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_cap/activity-leases/ibmon-keepalive":
			events = append(events, "put_lease:m_cap")
			exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_cap","expiresAt":"` + exp + `"}`))

		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_idle":
			v := vms["m_idle"]
			_, _ = w.Write([]byte(`{"id":"m_idle","name":"vm-idle","state":"` + v.state + `","desiredState":"` + v.state + `","autoSuspendedAt":"` + v.autoSuspendedAt + `","cooldownUntil":"` + v.cooldownUntil + `","maxRunMinutes":480,"free":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_idle/wake":
			events = append(events, "wake:m_idle")
			// Keep suspended so fallback /resume triggers, just like render_watch.py!
			_, _ = w.Write([]byte(`{"id":"m_idle","name":"vm-idle","state":"suspended","desiredState":"running","autoSuspendedAt":"` + vms["m_idle"].autoSuspendedAt + `"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_idle/resume":
			events = append(events, "resume:m_idle")
			vms["m_idle"].state = "running"
			vms["m_idle"].autoSuspendedAt = ""
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_res","machineId":"m_idle","state":"succeeded"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_idle/activity-leases/ibmon-keepalive":
			events = append(events, "put_lease:m_idle")
			exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_idle","expiresAt":"` + exp + `"}`))

		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_manual":
			v := vms["m_manual"]
			_, _ = w.Write([]byte(`{"id":"m_manual","name":"vm-manual","state":"` + v.state + `","desiredState":"` + v.state + `","maxRunMinutes":480,"free":true}`))
		case r.Method == http.MethodPost && (r.URL.Path == "/v1/machines/m_manual/start" || r.URL.Path == "/v1/machines/m_manual/wake" || r.URL.Path == "/v1/machines/m_manual/resume"):
			events = append(events, "unexpected_wake_manual:"+r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_lease", Name: "vm-lease", State: "running", DesiredState: "running", MaxRunMinutes: 480},
		{AccountID: acc.ID, MachineID: "m_cap", Name: "vm-cap", State: "running", DesiredState: "running", MaxRunMinutes: 480},
		{AccountID: acc.ID, MachineID: "m_idle", Name: "vm-idle", State: "suspended", DesiredState: "suspended", AutoSuspendedAt: vms["m_idle"].autoSuspendedAt, MaxRunMinutes: 480},
		{AccountID: acc.ID, MachineID: "m_manual", Name: "vm-manual", State: "stopped", DesiredState: "stopped", MaxRunMinutes: 480},
	})
	for _, mid := range []string{"m_lease", "m_cap", "m_idle", "m_manual"} {
		if err := st.SetMachineKeepalive(context.Background(), acc.ID, mid, true); err != nil {
			t.Fatalf("SetMachineKeepalive(%s): %v", mid, err)
		}
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	p.SetKeepalivePollDelay(time.Millisecond)
	defer p.Stop()

	ctx := context.Background()
	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_lease", true); err != nil {
		t.Fatalf("m_lease cycle: %v", err)
	}
	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_cap", false); err != nil {
		t.Fatalf("m_cap cycle: %v", err)
	}
	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_idle", false); err != nil {
		t.Fatalf("m_idle cycle: %v", err)
	}
	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_manual", false); err != nil {
		t.Fatalf("m_manual cycle: %v", err)
	}

	// Disabling keepalive on m_lease should release its activity lease
	if _, err := p.SetMachineKeepalive(ctx, acc.ID, "m_lease", false); err != nil {
		t.Fatalf("SetMachineKeepalive(false): %v", err)
	}

	mu.Lock()
	gotEvents := append([]string(nil), events...)
	mu.Unlock()

	want := []string{
		"put_lease:m_lease",
		"stop:m_cap", "start:m_cap", "put_lease:m_cap",
		"wake:m_idle", "resume:m_idle", "put_lease:m_idle",
		"delete_lease:m_lease",
	}
	if len(gotEvents) != len(want) {
		t.Fatalf("events mismatch:\n got: %v\nwant: %v", gotEvents, want)
	}
	for i := range want {
		if gotEvents[i] != want[i] {
			t.Fatalf("event[%d] mismatch: got %q want %q (all=%v)", i, gotEvents[i], want[i], gotEvents)
		}
	}

	capRec, _ := st.GetMachine(ctx, acc.ID, "m_cap")
	if capRec.KeepaliveLastAction != "power_cycle" || capRec.State != "running" {
		t.Fatalf("expected m_cap power_cycle running, got %+v", capRec)
	}
	manualRec, _ := st.GetMachine(ctx, acc.ID, "m_manual")
	if manualRec.KeepaliveLastAction != "manual_paused" {
		t.Fatalf("expected m_manual preserved with manual_paused, got %+v", manualRec)
	}

	// Verify active cooldown prevents /wake spam
	mu.Lock()
	vms["m_idle"].state = "suspended"
	vms["m_idle"].autoSuspendedAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)
	vms["m_idle"].cooldownUntil = time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
	beforeLen := len(events)
	mu.Unlock()

	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_idle", false); err != nil {
		t.Fatalf("m_idle cooldown cycle: %v", err)
	}
	mu.Lock()
	afterLen := len(events)
	mu.Unlock()
	if afterLen != beforeLen {
		t.Fatalf("expected zero wake/resume requests during cooldown, got %v", events[beforeLen:])
	}
	cdRec, _ := st.GetMachine(ctx, acc.ID, "m_idle")
	if cdRec.KeepaliveLastAction != "cooldown_wait" {
		t.Fatalf("expected cooldown_wait action, got %+v", cdRec)
	}
}

func TestKeepaliveDisableCancelsAndFencesActiveCycle(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	wokeCh := make(chan struct{}, 1)
	var mu sync.Mutex
	var events []string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_race":
			_, _ = w.Write([]byte(`{"id":"m_race","name":"vm-race","state":"suspended","desiredState":"suspended","autoSuspendedAt":"2026-11-13T11:00:00Z","maxRunMinutes":480,"free":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_race/wake":
			mu.Lock()
			events = append(events, "wake")
			mu.Unlock()
			select {
			case wokeCh <- struct{}{}:
			default:
			}
			_, _ = w.Write([]byte(`{"id":"m_race","name":"vm-race","state":"suspended","desiredState":"running","autoSuspendedAt":"2026-11-13T11:00:00Z"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_race/resume":
			mu.Lock()
			events = append(events, "resume")
			mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_res","machineId":"m_race","state":"succeeded"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_race/activity-leases/ibmon-keepalive":
			mu.Lock()
			events = append(events, "put_lease")
			mu.Unlock()
			exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_race","expiresAt":"` + exp + `"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_race/activity-leases/ibmon-keepalive":
			mu.Lock()
			events = append(events, "delete_lease")
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_race", Name: "vm-race", State: "suspended", DesiredState: "suspended", AutoSuspendedAt: "2026-11-13T11:00:00Z", MaxRunMinutes: 480},
	})
	if err := st.SetMachineKeepalive(context.Background(), acc.ID, "m_race", true); err != nil {
		t.Fatalf("SetMachineKeepalive(true): %v", err)
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	p.SetKeepalivePollDelay(150 * time.Millisecond)
	defer p.Stop()

	ctx := context.Background()
	cycleDone := make(chan error, 1)
	go func() {
		cycleDone <- p.RunKeepaliveCycle(ctx, acc.ID, "m_race", false)
	}()

	select {
	case <-wokeCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for /wake")
	}

	start := time.Now()
	rec, err := p.SetMachineKeepalive(ctx, acc.ID, "m_race", false)
	if err != nil {
		t.Fatalf("SetMachineKeepalive(false): %v", err)
	}
	if rec.KeepaliveEnabled {
		t.Fatalf("expected keepalive disabled immediately, got %+v", rec)
	}

	select {
	case <-cycleDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected active RunKeepaliveCycle to cancel quickly on SetMachineKeepalive(false), elapsed=%v", time.Since(start))
	}

	mu.Lock()
	gotEvents := append([]string(nil), events...)
	mu.Unlock()
	for _, ev := range gotEvents {
		if ev == "resume" || ev == "put_lease" {
			t.Fatalf("cancelled keepalive cycle continued to execute %q: events=%v", ev, gotEvents)
		}
	}
	if len(gotEvents) != 2 || gotEvents[0] != "wake" || gotEvents[1] != "delete_lease" {
		t.Fatalf("unexpected events: got=%v want=[wake delete_lease]", gotEvents)
	}

	stored, err := st.GetMachine(ctx, acc.ID, "m_race")
	if err != nil || stored.KeepaliveEnabled || stored.KeepaliveLeaseExpiresAt != 0 || stored.KeepaliveLastAt != 0 || stored.KeepaliveLastAction != "" || stored.KeepaliveLastError != "" {
		t.Fatalf("expected cleared keepalive DB fields after cancel, got err=%v m=%+v", err, stored)
	}
}

func TestKeepaliveDisableFencesBlockedLockAndRapidDisableEnableRunsForceLease(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	metricsEntered := make(chan struct{}, 1)
	metricsRelease := make(chan struct{})
	getEntered := make(chan struct{}, 4)

	var mu sync.Mutex
	var events []string
	var leasePutCount atomic.Int32
	leasePutDone := make(chan struct{}, 4)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_fast","name":"vm-fast","state":"running","desiredState":"running","runStartedAt":"` + time.Now().UTC().Add(-10*time.Minute).Format(time.RFC3339) + `","maxRunMinutes":480,"free":true}],"nextCursor":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_fast/usage":
			_, _ = w.Write([]byte(`{"runningSeconds":600}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_fast/metrics":
			select {
			case metricsEntered <- struct{}{}:
			default:
			}
			select {
			case <-metricsRelease:
			case <-r.Context().Done():
			}
			_, _ = w.Write([]byte(`{"metrics":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_fast/disk":
			_, _ = w.Write([]byte(`{"measuredAt":"2026-11-13T12:00:00Z","filesystems":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_fast":
			select {
			case getEntered <- struct{}{}:
			default:
			}
			_, _ = w.Write([]byte(`{"id":"m_fast","name":"vm-fast","state":"running","desiredState":"running","runStartedAt":"` + time.Now().UTC().Add(-10*time.Minute).Format(time.RFC3339) + `","maxRunMinutes":480,"free":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_fast/activity-leases/ibmon-keepalive":
			mu.Lock()
			events = append(events, "put_lease")
			mu.Unlock()
			leasePutCount.Add(1)
			select {
			case leasePutDone <- struct{}{}:
			default:
			}
			exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_fast","expiresAt":"` + exp + `"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_fast/activity-leases/ibmon-keepalive":
			mu.Lock()
			events = append(events, "delete_lease")
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_fast", Name: "vm-fast", State: "running", DesiredState: "running", MaxRunMinutes: 480},
	})
	if err := st.SetMachineKeepalive(context.Background(), acc.ID, "m_fast", true); err != nil {
		t.Fatalf("SetMachineKeepalive(true): %v", err)
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	p.SetKeepalivePollDelay(10 * time.Millisecond)
	defer p.Stop()

	ctx := context.Background()
	syncDone := make(chan error, 1)
	go func() {
		syncDone <- p.SyncAccount(ctx, acc.ID)
	}()

	select {
	case <-metricsEntered:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for SyncAccount to enter /metrics")
	}

	// Start RunKeepaliveCycle while SyncAccount holds accountLock; it will block at syncSingleMachine.
	oldCycleDone := make(chan error, 1)
	go func() {
		oldCycleDone <- p.RunKeepaliveCycle(ctx, acc.ID, "m_fast", false)
	}()

	select {
	case <-getEntered:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for RunKeepaliveCycle GetMachine")
	}
	time.Sleep(25 * time.Millisecond)

	// Disable while old cycle is blocked on accountLock, then immediately re-enable (forceLease=true).
	disableDone := make(chan error, 1)
	go func() {
		_, err := p.SetMachineKeepalive(ctx, acc.ID, "m_fast", false)
		disableDone <- err
	}()

	select {
	case err := <-disableDone:
		if err != nil {
			t.Fatalf("SetMachineKeepalive(false) err = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		close(metricsRelease)
		t.Fatalf("SetMachineKeepalive(false) blocked on lockAccount-held SyncAccount instead of cancelling blocked cycle")
	}

	select {
	case <-oldCycleDone:
	case <-time.After(500 * time.Millisecond):
		close(metricsRelease)
		t.Fatalf("old RunKeepaliveCycle did not exit promptly after disable")
	}

	// Rapid re-enable MUST NOT be dropped; it should acquire the lease once accountLock clears.
	if _, err := p.SetMachineKeepalive(ctx, acc.ID, "m_fast", true); err != nil {
		close(metricsRelease)
		t.Fatalf("SetMachineKeepalive(true) err = %v", err)
	}

	close(metricsRelease)
	select {
	case err := <-syncDone:
		if err != nil {
			t.Fatalf("SyncAccount err = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("SyncAccount did not finish")
	}

	select {
	case <-leasePutDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("rapid disable->enable dropped forceLease cycle (PutActivityLease never called)")
	}

	// Wait briefly to ensure old cycle didn't put a duplicate lease
	time.Sleep(30 * time.Millisecond)
	if leasePutCount.Load() != 1 {
		t.Fatalf("expected exactly 1 PutActivityLease from new enabled cycle, got %d", leasePutCount.Load())
	}
	mu.Lock()
	gotEvents := append([]string(nil), events...)
	mu.Unlock()
	if len(gotEvents) != 2 || gotEvents[0] != "delete_lease" || gotEvents[1] != "put_lease" {
		t.Fatalf("expected [delete_lease put_lease], got %v", gotEvents)
	}

	stored, err := st.GetMachine(ctx, acc.ID, "m_fast")
	if err != nil || !stored.KeepaliveEnabled || stored.KeepaliveLastAction != "lease_renewed" || stored.KeepaliveLeaseExpiresAt <= 0 {
		t.Fatalf("expected machine keepalive active with renewed lease, got err=%v m=%+v", err, stored)
	}
}

func TestKeepalivePowerCycleRecoveryAndShorterFreeMaxRunThreshold(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	type vmSim struct {
		state        string
		runStartedAt string
		stopCount    int
		startCount   int
		failStarts   int
		stayStopping bool
		leasePutCh   chan struct{}
	}
	vms := map[string]*vmSim{
		"m_short": {
			state:        "running",
			runStartedAt: time.Now().UTC().Add(-7015 * time.Second).Format(time.RFC3339),
			failStarts:   1,
			leasePutCh:   make(chan struct{}, 8),
		},
		"m_stop_timeout": {
			state:        "running",
			runStartedAt: time.Now().UTC().Add(-7025 * time.Second).Format(time.RFC3339),
			stayStopping: true,
			leasePutCh:   make(chan struct{}, 8),
		},
		"m_deadline": {
			state:        "running",
			runStartedAt: time.Now().UTC().Add(-7025 * time.Second).Format(time.RFC3339),
			stayStopping: true,
			leasePutCh:   make(chan struct{}, 8),
		},
	}

	var mu sync.Mutex
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":3}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
			_, _ = w.Write([]byte(`{"machines":[` +
				`{"id":"m_short","name":"vm-short","state":"` + vms["m_short"].state + `","desiredState":"` + vms["m_short"].state + `","runStartedAt":"` + vms["m_short"].runStartedAt + `","maxRunMinutes":120,"free":true},` +
				`{"id":"m_stop_timeout","name":"vm-stoptmo","state":"` + vms["m_stop_timeout"].state + `","desiredState":"` + vms["m_stop_timeout"].state + `","runStartedAt":"` + vms["m_stop_timeout"].runStartedAt + `","maxRunMinutes":120,"free":true},` +
				`{"id":"m_deadline","name":"vm-deadline","state":"` + vms["m_deadline"].state + `","desiredState":"` + vms["m_deadline"].state + `","runStartedAt":"` + vms["m_deadline"].runStartedAt + `","maxRunMinutes":120,"free":true}` +
				`],"nextCursor":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_short/usage",
			r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_stop_timeout/usage",
			r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_deadline/usage":
			_, _ = w.Write([]byte(`{"runningSeconds":100}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_short/metrics",
			r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_stop_timeout/metrics",
			r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_deadline/metrics":
			_, _ = w.Write([]byte(`{"metrics":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_short/disk",
			r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_stop_timeout/disk",
			r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_deadline/disk":
			_, _ = w.Write([]byte(`{"measuredAt":"2026-11-13T12:00:00Z","filesystems":[]}`))
		default:
			for id, v := range vms {
				base := "/v1/machines/" + id
				switch {
				case r.Method == http.MethodGet && r.URL.Path == base:
					_, _ = w.Write([]byte(`{"id":"` + id + `","name":"` + id + `","state":"` + v.state + `","desiredState":"` + v.state + `","runStartedAt":"` + v.runStartedAt + `","maxRunMinutes":120,"free":true}`))
					return
				case r.Method == http.MethodPost && r.URL.Path == base+"/stop":
					v.stopCount++
					if v.stayStopping {
						v.state = "stopping"
					} else {
						v.state = "stopped"
					}
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(`{"id":"op_stop","machineId":"` + id + `","state":"succeeded"}`))
					return
				case r.Method == http.MethodPost && r.URL.Path == base+"/start":
					v.startCount++
					if v.failStarts > 0 {
						v.failStarts--
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"error":{"code":"transient_error","message":"start temporarily failed"}}`))
						return
					}
					v.state = "running"
					v.runStartedAt = time.Now().UTC().Format(time.RFC3339)
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(`{"id":"op_start","machineId":"` + id + `","state":"succeeded"}`))
					return
				case r.Method == http.MethodPut && r.URL.Path == base+"/activity-leases/ibmon-keepalive":
					exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
					_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"` + id + `","expiresAt":"` + exp + `"}`))
					select {
					case v.leasePutCh <- struct{}{}:
					default:
					}
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_short", Name: "vm-short", State: "running", DesiredState: "running", MaxRunMinutes: 120},
		{AccountID: acc.ID, MachineID: "m_stop_timeout", Name: "vm-stoptmo", State: "running", DesiredState: "running", MaxRunMinutes: 120},
		{AccountID: acc.ID, MachineID: "m_deadline", Name: "vm-deadline", State: "running", DesiredState: "running", MaxRunMinutes: 120},
	})
	for _, id := range []string{"m_short", "m_stop_timeout", "m_deadline"} {
		if err := st.SetMachineKeepalive(context.Background(), acc.ID, id, true); err != nil {
			t.Fatalf("SetMachineKeepalive(%s): %v", id, err)
		}
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	p.SetKeepalivePollDelay(2 * time.Millisecond)
	defer p.Stop()

	ctx := context.Background()

	// 1. At 7015s (< 7020s threshold for maxRunMinutes=120), RunKeepaliveCycle only puts lease, no stop!
	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_short", true); err != nil {
		t.Fatalf("m_short below threshold cycle: %v", err)
	}
	mu.Lock()
	if vms["m_short"].stopCount != 0 {
		mu.Unlock()
		t.Fatalf("expected 0 stops at 7015s (<7020s), got %d", vms["m_short"].stopCount)
	}
	// Advance runtime to exactly 7020s (117m for maxRunMinutes=120) -> triggers powerCycleForKeepalive; ActionStart fails once.
	vms["m_short"].runStartedAt = time.Now().UTC().Add(-7020 * time.Second).Format(time.RFC3339)
	mu.Unlock()

	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_short", false); err == nil {
		t.Fatalf("expected transient ActionStart failure error on m_short")
	}
	shortRec, _ := st.GetMachine(ctx, acc.ID, "m_short")
	if !shortRec.KeepalivePendingRestart || shortRec.State != "stopped" {
		t.Fatalf("expected KeepalivePendingRestart=true and state=stopped after start failure, got %+v", shortRec)
	}

	// 2. m_stop_timeout exceeds 20 stop poll iterations while "stopping", then transitions to "stopped".
	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_stop_timeout", false); err == nil {
		t.Fatalf("expected stop timeout error on m_stop_timeout")
	}
	stopTmoRec, _ := st.GetMachine(ctx, acc.ID, "m_stop_timeout")
	if !stopTmoRec.KeepalivePendingRestart {
		t.Fatalf("expected KeepalivePendingRestart=true after stop poll timeout, got %+v", stopTmoRec)
	}

	// 3. m_deadline hits context deadline during stop polling.
	shortCtx, cancel := context.WithTimeout(ctx, 12*time.Millisecond)
	_ = p.RunKeepaliveCycle(shortCtx, acc.ID, "m_deadline", false)
	cancel()
	deadlineRec, _ := st.GetMachine(ctx, acc.ID, "m_deadline")
	if !deadlineRec.KeepalivePendingRestart {
		t.Fatalf("expected KeepalivePendingRestart=true after context deadline expiry, got %+v", deadlineRec)
	}

	// Transition m_stop_timeout and m_deadline to "stopped" (finished stopping upstream) and run SyncAccount -> ALL 3 must recover!
	mu.Lock()
	vms["m_stop_timeout"].stayStopping = false
	vms["m_stop_timeout"].state = "stopped"
	vms["m_deadline"].stayStopping = false
	vms["m_deadline"].state = "stopped"
	mu.Unlock()

	if err := p.SyncAccount(ctx, acc.ID); err != nil {
		t.Fatalf("SyncAccount recovery err: %v", err)
	}

	for _, id := range []string{"m_short", "m_stop_timeout", "m_deadline"} {
		select {
		case <-vms[id].leasePutCh:
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %s recovery to acquire lease", id)
		}
		// Give updateKeepaliveState a brief moment after PutActivityLease
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			rec, _ := st.GetMachine(ctx, acc.ID, id)
			if rec.State == "running" && !rec.KeepalivePendingRestart && rec.KeepaliveLastAction == "power_cycle" && rec.KeepaliveLastError == "" {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		rec, _ := st.GetMachine(ctx, acc.ID, id)
		if rec.State != "running" || rec.KeepalivePendingRestart || rec.KeepaliveLastAction != "power_cycle" || rec.KeepaliveLastError != "" {
			t.Fatalf("expected %s fully recovered via power_cycle, got %+v", id, rec)
		}
	}
}

func TestExecuteActionStopClearsKeepalivePendingRestart(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var mu sync.Mutex
	vmState := "running"
	runStartedAt := time.Now().UTC().Add(-7030 * time.Second).Format(time.RFC3339)
	startCalls := 0

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_stop","name":"vm-stop","state":"` + vmState + `","desiredState":"` + vmState + `","runStartedAt":"` + runStartedAt + `","maxRunMinutes":120,"free":true}],"nextCursor":null}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_stop":
			_, _ = w.Write([]byte(`{"id":"m_stop","name":"vm-stop","state":"` + vmState + `","desiredState":"` + vmState + `","runStartedAt":"` + runStartedAt + `","maxRunMinutes":120,"free":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_stop/usage":
			_, _ = w.Write([]byte(`{"runningSeconds":100}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_stop/metrics":
			_, _ = w.Write([]byte(`{"metrics":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_stop/stop":
			vmState = "stopped"
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_stop","machineId":"m_stop","state":"succeeded"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_stop/start":
			startCalls++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"transient_error","message":"start failed"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_stop", Name: "vm-stop", State: "running", DesiredState: "running", MaxRunMinutes: 120},
	})
	if err := st.SetMachineKeepalive(context.Background(), acc.ID, "m_stop", true); err != nil {
		t.Fatalf("SetMachineKeepalive(true): %v", err)
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	p.SetKeepalivePollDelay(time.Millisecond)
	defer p.Stop()

	ctx := context.Background()
	if err := p.RunKeepaliveCycle(ctx, acc.ID, "m_stop", false); err == nil {
		t.Fatalf("expected ActionStart failure")
	}
	rec, _ := st.GetMachine(ctx, acc.ID, "m_stop")
	if !rec.KeepalivePendingRestart {
		t.Fatalf("expected KeepalivePendingRestart=true before manual stop, got %+v", rec)
	}

	if _, err := p.ExecuteAction(ctx, acc.ID, "m_stop", instabox.ActionStop); err != nil {
		t.Fatalf("ExecuteAction(ActionStop) err: %v", err)
	}
	rec, _ = st.GetMachine(ctx, acc.ID, "m_stop")
	if rec.KeepalivePendingRestart {
		t.Fatalf("expected ExecuteAction(ActionStop) to clear KeepalivePendingRestart, got %+v", rec)
	}

	mu.Lock()
	callsBefore := startCalls
	mu.Unlock()
	if err := p.SyncAccount(ctx, acc.ID); err != nil {
		t.Fatalf("SyncAccount err: %v", err)
	}
	time.Sleep(40 * time.Millisecond)

	mu.Lock()
	callsAfter := startCalls
	mu.Unlock()
	if callsAfter != callsBefore {
		t.Fatalf("expected stopped VM after explicit ActionStop NOT to retry start, got %d new starts", callsAfter-callsBefore)
	}
	rec, _ = st.GetMachine(ctx, acc.ID, "m_stop")
	if rec.KeepaliveLastAction != "manual_paused" {
		t.Fatalf("expected manual_paused after explicit ActionStop, got %+v", rec)
	}
}

func TestSyncAccountReconcilesStaleKeepaliveMetadataWhenSkipped(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var actionCalls atomic.Int32
	cdUntil := time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339)
	autoSusp := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
			_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":3}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
			_, _ = w.Write([]byte(`{"machines":[` +
				`{"id":"m_stop","name":"vm-stop","state":"stopped","desiredState":"stopped","maxRunMinutes":480,"free":true},` +
				`{"id":"m_pause","name":"vm-pause","state":"suspended","desiredState":"suspended","maxRunMinutes":480,"free":true},` +
				`{"id":"m_cd","name":"vm-cd","state":"suspended","desiredState":"suspended","autoSuspendedAt":"` + autoSusp + `","cooldownUntil":"` + cdUntil + `","maxRunMinutes":480,"free":true}` +
				`],"nextCursor":null}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/v1/machines/m_stop/usage" || r.URL.Path == "/v1/machines/m_pause/usage" || r.URL.Path == "/v1/machines/m_cd/usage"):
			_, _ = w.Write([]byte(`{"runningSeconds":100}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/v1/machines/m_stop/metrics" || r.URL.Path == "/v1/machines/m_pause/metrics" || r.URL.Path == "/v1/machines/m_cd/metrics"):
			_, _ = w.Write([]byte(`{"metrics":[]}`))
		default:
			actionCalls.Add(1)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_stop", Name: "vm-stop", State: "running", DesiredState: "running", MaxRunMinutes: 480},
		{AccountID: acc.ID, MachineID: "m_pause", Name: "vm-pause", State: "running", DesiredState: "running", MaxRunMinutes: 480},
		{AccountID: acc.ID, MachineID: "m_cd", Name: "vm-cd", State: "running", DesiredState: "running", MaxRunMinutes: 480},
	})
	ctx := context.Background()
	futureExp := time.Now().UTC().Add(50 * time.Minute).Unix()
	for _, id := range []string{"m_stop", "m_pause", "m_cd"} {
		if err := st.SetMachineKeepalive(ctx, acc.ID, id, true); err != nil {
			t.Fatalf("SetMachineKeepalive(%s): %v", id, err)
		}
		if err := st.UpdateMachineKeepaliveState(ctx, acc.ID, id, futureExp, time.Now().Unix(), "lease_renewed", "stale err"); err != nil {
			t.Fatalf("UpdateMachineKeepaliveState(%s): %v", id, err)
		}
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	defer p.Stop()

	if err := p.SyncAccount(ctx, acc.ID); err != nil {
		t.Fatalf("SyncAccount err: %v", err)
	}
	if actionCalls.Load() != 0 {
		t.Fatalf("expected 0 upstream keepalive action calls, got %d", actionCalls.Load())
	}

	stopRec, _ := st.GetMachine(ctx, acc.ID, "m_stop")
	if stopRec.KeepaliveLeaseExpiresAt != 0 || stopRec.KeepaliveLastAction != "manual_paused" || stopRec.KeepaliveLastError != "" {
		t.Fatalf("expected m_stop reconciled to manual_paused with lease=0, got %+v", stopRec)
	}
	pauseRec, _ := st.GetMachine(ctx, acc.ID, "m_pause")
	if pauseRec.KeepaliveLeaseExpiresAt != 0 || pauseRec.KeepaliveLastAction != "manual_paused" || pauseRec.KeepaliveLastError != "" {
		t.Fatalf("expected m_pause reconciled to manual_paused with lease=0, got %+v", pauseRec)
	}
	cdRec, _ := st.GetMachine(ctx, acc.ID, "m_cd")
	if cdRec.KeepaliveLeaseExpiresAt != 0 || cdRec.KeepaliveLastAction != "cooldown_wait" || cdRec.KeepaliveLastError != "" {
		t.Fatalf("expected m_cd reconciled to cooldown_wait with lease=0, got %+v", cdRec)
	}
}

func TestKeepaliveWakeDoesNotFallbackToResumeOnTransitionAndMapsResumeCooldown(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	autoSusp := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	cdUntil := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)

	type vmSim struct {
		state           string
		autoSuspendedAt string
		cooldownUntil   string
		afterWakeState  string
		afterWakeAuto   string
		afterWakeCD     string
		resumeCooldown  bool
	}
	vms := map[string]*vmSim{
		"m_to_stopped": {
			state:           "suspended",
			autoSuspendedAt: autoSusp,
			afterWakeState:  "stopped",
		},
		"m_to_manual": {
			state:           "suspended",
			autoSuspendedAt: autoSusp,
			afterWakeState:  "suspended",
			afterWakeAuto:   "",
		},
		"m_to_cooldown": {
			state:           "suspended",
			autoSuspendedAt: autoSusp,
			afterWakeState:  "suspended",
			afterWakeAuto:   autoSusp,
			afterWakeCD:     cdUntil,
		},
		"m_resume_cd": {
			state:           "suspended",
			autoSuspendedAt: autoSusp,
			afterWakeState:  "suspended",
			afterWakeAuto:   autoSusp,
			resumeCooldown:  true,
		},
	}

	var mu sync.Mutex
	var events []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		mu.Lock()
		defer mu.Unlock()
		for id, v := range vms {
			base := "/v1/machines/" + id
			switch {
			case r.Method == http.MethodGet && r.URL.Path == base:
				_, _ = w.Write([]byte(`{"id":"` + id + `","name":"` + id + `","state":"` + v.state + `","desiredState":"` + v.state + `","autoSuspendedAt":"` + v.autoSuspendedAt + `","cooldownUntil":"` + v.cooldownUntil + `","maxRunMinutes":480,"free":true}`))
				return
			case r.Method == http.MethodPost && r.URL.Path == base+"/wake":
				events = append(events, "wake:"+id)
				v.state = v.afterWakeState
				v.autoSuspendedAt = v.afterWakeAuto
				v.cooldownUntil = v.afterWakeCD
				_, _ = w.Write([]byte(`{"id":"` + id + `","name":"` + id + `","state":"suspended","desiredState":"running","autoSuspendedAt":"` + autoSusp + `"}`))
				return
			case r.Method == http.MethodPost && r.URL.Path == base+"/resume":
				events = append(events, "resume:"+id)
				if v.resumeCooldown {
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`{"error":{"code":"machine_cooldown","message":"machine in cooldown"}}`))
					return
				}
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"id":"op_res","machineId":"` + id + `","state":"succeeded"}`))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rt{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
	var recs []store.MachineRecord
	for id := range vms {
		recs = append(recs, store.MachineRecord{
			AccountID: acc.ID, MachineID: id, Name: id, State: "suspended", DesiredState: "suspended", AutoSuspendedAt: autoSusp, MaxRunMinutes: 480,
		})
	}
	_ = st.SyncAccountMachines(context.Background(), acc.ID, recs)
	ctx := context.Background()
	for id := range vms {
		if err := st.SetMachineKeepalive(ctx, acc.ID, id, true); err != nil {
			t.Fatalf("SetMachineKeepalive(%s): %v", id, err)
		}
	}

	p := monitor.NewPoller(st, client, 30*time.Second, 7)
	p.SetKeepalivePollDelay(time.Millisecond)
	defer p.Stop()

	for _, id := range []string{"m_to_stopped", "m_to_manual", "m_to_cooldown", "m_resume_cd"} {
		if err := p.RunKeepaliveCycle(ctx, acc.ID, id, false); err != nil {
			t.Fatalf("RunKeepaliveCycle(%s) unexpected error: %v", id, err)
		}
	}

	mu.Lock()
	gotEvents := append([]string(nil), events...)
	mu.Unlock()
	wantEvents := []string{
		"wake:m_to_stopped",
		"wake:m_to_manual",
		"wake:m_to_cooldown",
		"wake:m_resume_cd",
		"resume:m_resume_cd",
	}
	if len(gotEvents) != len(wantEvents) {
		t.Fatalf("events mismatch:\n got: %v\nwant: %v", gotEvents, wantEvents)
	}
	for i := range wantEvents {
		if gotEvents[i] != wantEvents[i] {
			t.Fatalf("event[%d] mismatch: got %q want %q (all=%v)", i, gotEvents[i], wantEvents[i], gotEvents)
		}
	}

	rStopped, _ := st.GetMachine(ctx, acc.ID, "m_to_stopped")
	if rStopped.KeepaliveLastAction != "manual_paused" || rStopped.KeepaliveLeaseExpiresAt != 0 {
		t.Fatalf("expected m_to_stopped -> manual_paused, got %+v", rStopped)
	}
	rManual, _ := st.GetMachine(ctx, acc.ID, "m_to_manual")
	if rManual.KeepaliveLastAction != "manual_paused" || rManual.KeepaliveLeaseExpiresAt != 0 {
		t.Fatalf("expected m_to_manual -> manual_paused, got %+v", rManual)
	}
	rCD, _ := st.GetMachine(ctx, acc.ID, "m_to_cooldown")
	if rCD.KeepaliveLastAction != "cooldown_wait" || rCD.KeepaliveLeaseExpiresAt != 0 {
		t.Fatalf("expected m_to_cooldown -> cooldown_wait, got %+v", rCD)
	}
	rResCD, _ := st.GetMachine(ctx, acc.ID, "m_resume_cd")
	if rResCD.KeepaliveLastAction != "cooldown_wait" || rResCD.KeepaliveLastError != "" || rResCD.KeepaliveLeaseExpiresAt != 0 {
		t.Fatalf("expected m_resume_cd -> cooldown_wait without error, got %+v", rResCD)
	}
}

func TestKeepaliveLeaseRenewalUsesUpstreamClockUnderHostClockSkew(t *testing.T) {
	runCase := func(t *testing.T, skew time.Duration, leaseTTLRemaining time.Duration, expectRenew bool) {
		t.Helper()
		dir := t.TempDir()
		key, _ := crypto.LoadOrCreateMasterKey(dir, "")
		st, _ := store.Open(filepath.Join(dir, "mon.db"), key)
		t.Cleanup(func() { _ = st.Close() })

		upstreamBase := time.Now().UTC().Add(skew)
		runStarted := upstreamBase.Add(-5 * time.Minute).Format(time.RFC3339)

		var putCount atomic.Int32
		putCh := make(chan struct{}, 4)

		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Date", upstreamBase.Format(http.TimeFormat))
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
				_, _ = w.Write([]byte(`{"userId":1,"login":"u","org":{"id":"org_1","name":"P","tier":"free","kind":"personal"}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/v1/usage":
				_, _ = w.Write([]byte(`{"machines":1}`))
			case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
				_, _ = w.Write([]byte(`{"machines":[{"id":"m_skew","name":"vm-skew","state":"running","desiredState":"running","runStartedAt":"` + runStarted + `","maxRunMinutes":480,"free":true}],"nextCursor":null}`))
			case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_skew":
				_, _ = w.Write([]byte(`{"id":"m_skew","name":"vm-skew","state":"running","desiredState":"running","runStartedAt":"` + runStarted + `","maxRunMinutes":480,"free":true}`))
			case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_skew/usage":
				_, _ = w.Write([]byte(`{"runningSeconds":300}`))
			case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_skew/metrics":
				_, _ = w.Write([]byte(`{"metrics":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_skew/disk":
				_, _ = w.Write([]byte(`{"measuredAt":"2026-11-13T12:00:00Z","filesystems":[]}`))
			case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_skew/activity-leases/ibmon-keepalive":
				n := putCount.Add(1)
				exp := upstreamBase.Add(leaseTTLRemaining).Format(time.RFC3339)
				if n > 1 {
					exp = upstreamBase.Add(time.Hour).Format(time.RFC3339)
				}
				_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_skew","expiresAt":"` + exp + `"}`))
				select {
				case putCh <- struct{}{}:
				default:
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		hc := srv.Client()
		hc.Transport = &rt{u: u, t: srv.Client().Transport}
		client, _ := instabox.NewClient("https://api.instabox.sh", hc)

		acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "A", APIKey: "ibk_a...9", OrgTier: "free"})
		_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
			{AccountID: acc.ID, MachineID: "m_skew", Name: "vm-skew", State: "running", DesiredState: "running", RunStartedAt: runStarted, MaxRunMinutes: 480},
		})
		ctx := context.Background()
		if err := st.SetMachineKeepalive(ctx, acc.ID, "m_skew", true); err != nil {
			t.Fatalf("SetMachineKeepalive: %v", err)
		}

		p := monitor.NewPoller(st, client, 30*time.Second, 7)
		p.SetKeepalivePollDelay(time.Millisecond)
		defer p.Stop()

		// 1. Prime clockOffsetMs via SyncAccount (first sync sees runChanged -> puts initial lease expiring at upstreamBase + leaseTTLRemaining).
		if err := p.SyncAccount(ctx, acc.ID); err != nil {
			t.Fatalf("initial SyncAccount: %v", err)
		}
		select {
		case <-putCh:
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for initial PutActivityLease")
		}
		// Wait for UpdateMachineKeepaliveState after initial PutActivityLease
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			rec, _ := st.GetMachine(ctx, acc.ID, "m_skew")
			if rec.KeepaliveLeaseExpiresAt > 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		// 2. Second SyncAccount checks KeepaliveLeaseExpiresAt against upstreamNow().
		if err := p.SyncAccount(ctx, acc.ID); err != nil {
			t.Fatalf("second SyncAccount: %v", err)
		}
		if expectRenew {
			select {
			case <-putCh:
			case <-time.After(2 * time.Second):
				t.Fatalf("expected lease renewal when upstream clock is %v skewed and %v left on lease", skew, leaseTTLRemaining)
			}
		} else {
			time.Sleep(50 * time.Millisecond)
			if putCount.Load() != 1 {
				t.Fatalf("expected NO premature lease renewal when upstream clock is %v skewed and %v left on lease, got %d puts", skew, leaseTTLRemaining, putCount.Load())
			}
		}
	}

	t.Run("upstream_ahead_needs_renewal", func(t *testing.T) {
		runCase(t, 2*time.Hour, 30*time.Minute, true)
	})
	t.Run("upstream_behind_skips_premature_renewal", func(t *testing.T) {
		runCase(t, -2*time.Hour, 55*time.Minute, false)
	})
}
