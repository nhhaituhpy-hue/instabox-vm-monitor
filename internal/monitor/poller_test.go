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
