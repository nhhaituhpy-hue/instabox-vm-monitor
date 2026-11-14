package instabox_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
)

type rerouteTransport struct {
	target *url.URL
	inner  http.RoundTripper
}

func (rt *rerouteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := *clone.URL
	u.Scheme = rt.target.Scheme
	u.Host = rt.target.Host
	clone.URL = &u
	return rt.inner.RoundTrip(clone)
}

func newMockClient(t *testing.T, handler http.HandlerFunc) (*instabox.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &rerouteTransport{target: u, inner: srv.Client().Transport}
	c, err := instabox.NewClient("https://api.instabox.sh", hc)
	if err != nil {
		t.Fatalf("NewClient err = %v", err)
	}
	return c, srv
}

func TestListMachinesPaginationAndMetrics(t *testing.T) {
	var pageCalls atomic.Int32
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "ibk_test_key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_key","message":"bad key","retryable":false}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch r.URL.Path {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"userId":7,"login":"hainh","org":{"id":"org_1","name":"Personal","tier":"free","kind":"personal"}}`))
		case "/v1/machines":
			cursor := r.URL.Query().Get("cursor")
			pageCalls.Add(1)
			if cursor == "" {
				_, _ = w.Write([]byte(`{"machines":[{"id":"m_1","name":"box-a","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"maxRunMinutes":480,"free":true}],"nextCursor":"p2"}`))
				return
			}
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_2","name":"box-b","state":"suspended","desiredState":"suspended","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":false,"processesPreserved":true},"autoSuspendedAt":"2026-11-13T10:00:00Z","maxRunMinutes":480,"free":true}],"nextCursor":null}`))
		case "/v1/machines/m_1/metrics":
			_, _ = w.Write([]byte(`{"disk":{"measuredAt":"2026-11-13T10:00:00Z","filesystems":[{"mount":"/data","sizeBytes":21474836480,"usedBytes":4294967296,"availableBytes":17179869184,"usedPct":20,"inodesUsedPct":2,"level":"ok"}]},"metrics":[{"timestamp":"2026-11-13T10:00:00Z","cpuCount":4,"cpuUsedPct":31.5,"memUsed":2147483648,"memTotal":8589934592,"diskTotal":21474836480,"diskUsed":4294967296}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ctx := context.Background()
	me, err := client.GetMe(ctx, "ibk_test_key")
	if err != nil || me.Login != "hainh" || me.Org.Tier != "free" {
		t.Fatalf("GetMe unexpected: %+v err=%v", me, err)
	}

	machines, _, err := client.ListAllMachines(ctx, "ibk_test_key")
	if err != nil || len(machines) != 2 || pageCalls.Load() != 2 {
		t.Fatalf("ListAllMachines got len=%d calls=%d err=%v", len(machines), pageCalls.Load(), err)
	}

	metrics, err := client.GetMachineMetrics(ctx, "ibk_test_key", "m_1", time.Now().Add(-time.Hour))
	if err != nil || len(metrics.Metrics) != 1 || metrics.Disk == nil || len(metrics.Disk.Filesystems) != 1 {
		t.Fatalf("GetMachineMetrics unexpected: %+v err=%v", metrics, err)
	}
}

func TestActionEndpointsSendIdempotencyKeyAndParseErrors(t *testing.T) {
	var seenMethod, seenPath, seenIdem string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenIdem = r.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/machines/m_1/stop":
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "op_123", "machineId": "m_1", "state": "queued"})
		case "/v1/machines/m_1/wake":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"code":"machine_cooldown","message":"machine cooling down","requestId":"req_1","retryable":true,"operationId":null,"retryAfterSeconds":180}}`))
		}
	})

	ctx := context.Background()
	res, err := client.PerformAction(ctx, "ibk_test", "m_1", instabox.ActionStop)
	if err != nil {
		t.Fatalf("ActionStop err = %v", err)
	}
	if seenMethod != http.MethodPost || seenPath != "/v1/machines/m_1/stop" || seenIdem == "" {
		t.Fatalf("unexpected request: method=%s path=%s idem=%q", seenMethod, seenPath, seenIdem)
	}
	if res.Operation == nil || res.Operation.ID != "op_123" {
		t.Fatalf("expected operation op_123, got %+v", res)
	}

	_, err = client.PerformAction(ctx, "ibk_test", "m_1", instabox.ActionWake)
	var apiErr *instabox.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *instabox.APIError, got %v", err)
	}
	if apiErr.Code != "machine_cooldown" || apiErr.RetryAfterSeconds != 180 {
		t.Fatalf("unexpected apiErr: %+v", apiErr)
	}
}

func TestStrictURLValidationRejectsHTTPAndUnexpectedHost(t *testing.T) {
	if _, err := instabox.NewClient("http://api.instabox.sh", nil); err == nil {
		t.Fatalf("expected http scheme to be rejected")
	}
	if _, err := instabox.NewClient("https://169.254.169.254", nil); err == nil {
		t.Fatalf("expected private/metadata IP to be rejected")
	}
}

func TestClientRejectsRedirectsToProtectAPIKey(t *testing.T) {
	var leaked atomic.Int32
	evil := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" {
			leaked.Add(1)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer evil.Close()

	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/steal", http.StatusFound)
	})
	if _, err := client.GetMe(context.Background(), "ibk_secret_no_leak"); err == nil {
		t.Fatalf("expected redirect to be rejected")
	}
	if leaked.Load() != 0 {
		t.Fatalf("API key leaked across HTTP redirect")
	}
}

func TestListTemplatesAndCreateMachine(t *testing.T) {
	var seenIdem string
	var seenBody map[string]any
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/templates":
			_, _ = w.Write([]byte(`{"templates":[{"id":"base","version":"2026-10-07.1","arch":"x86_64","hidden":true,"creatable":false,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20}},{"id":"agent-browser","version":"2026-11-01.3","description":"Browser box","arch":"x86_64","creatable":true,"latest":true,"default":true,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16}},{"id":"android","version":"2026-11-02.7","description":"Android 14","arch":"x86_64","creatable":true,"latest":true,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":24}}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines":
			seenIdem = r.Header.Get("Idempotency-Key")
			_ = json.NewDecoder(r.Body).Decode(&seenBody)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_create_1","machineId":"m_new_1","type":"create","state":"queued","phase":"scheduling"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ctx := context.Background()
	tpls, err := client.ListTemplates(ctx, "ibk_test")
	if err != nil {
		t.Fatalf("ListTemplates err = %v", err)
	}
	if len(tpls) != 3 || !tpls[1].Default || !tpls[1].Latest || tpls[1].DefaultSpec.RootDiskGiB != 16 {
		t.Fatalf("unexpected templates: %+v", tpls)
	}

	res, err := client.CreateMachine(ctx, "ibk_test", instabox.CreateMachineInput{
		Name:       "my-box-2",
		TemplateID: "android",
	})
	if err != nil {
		t.Fatalf("CreateMachine err = %v", err)
	}
	if seenIdem == "" {
		t.Fatalf("expected Idempotency-Key header on POST /v1/machines")
	}
	if seenBody["name"] != "my-box-2" || seenBody["templateId"] != "android" {
		t.Fatalf("unexpected create payload: %+v", seenBody)
	}
	if _, hasVCPUs := seenBody["vcpus"]; hasVCPUs {
		t.Fatalf("expected zero vcpus to be omitted, got %+v", seenBody)
	}
	if res.Operation == nil || res.Operation.ID != "op_create_1" || res.Operation.MachineID != "m_new_1" {
		t.Fatalf("unexpected create result: %+v", res)
	}
}

func TestRenameAndDeleteMachineSendIdempotencyAndConfirmHeader(t *testing.T) {
	var patchIdem, deleteIdem, confirmHdr string
	var patchBody map[string]any
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/machines/m_1":
			patchIdem = r.Header.Get("Idempotency-Key")
			_ = json.NewDecoder(r.Body).Decode(&patchBody)
			_, _ = w.Write([]byte(`{"id":"m_1","name":"renamed-box","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_1":
			deleteIdem = r.Header.Get("Idempotency-Key")
			confirmHdr = r.Header.Get("X-Confirm-Delete")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_del_1","machineId":"m_1","type":"delete","state":"queued"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ctx := context.Background()
	m, err := client.RenameMachine(ctx, "ibk_test", "m_1", "renamed-box")
	if err != nil {
		t.Fatalf("RenameMachine err = %v", err)
	}
	if patchIdem == "" || patchBody["name"] != "renamed-box" || m.Name != "renamed-box" {
		t.Fatalf("unexpected rename state: idem=%q body=%+v m=%+v", patchIdem, patchBody, m)
	}

	res, err := client.DeleteMachine(ctx, "ibk_test", "m_1", "m_1")
	if err != nil {
		t.Fatalf("DeleteMachine err = %v", err)
	}
	if deleteIdem == "" || confirmHdr != "m_1" || res.Operation == nil || res.Operation.ID != "op_del_1" {
		t.Fatalf("unexpected delete state: idem=%q confirm=%q res=%+v", deleteIdem, confirmHdr, res)
	}
}

func TestPutAndDeleteActivityLease(t *testing.T) {
	var putTTL float64
	var deleted atomic.Bool
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_1/activity-leases/ibmon-keepalive":
			var body map[string]float64
			_ = json.NewDecoder(r.Body).Decode(&body)
			putTTL = body["ttlSeconds"]
			_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_1","expiresAt":"2026-11-14T03:00:00Z"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_1/activity-leases/ibmon-keepalive":
			deleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ctx := context.Background()
	lease, err := client.PutActivityLease(ctx, "ibk_test", "m_1", "ibmon-keepalive", 3600)
	if err != nil {
		t.Fatalf("PutActivityLease err = %v", err)
	}
	if putTTL != 3600 || lease.ID != "ibmon-keepalive" || lease.ExpiresAt != "2026-11-14T03:00:00Z" {
		t.Fatalf("unexpected lease result: ttl=%v lease=%+v", putTTL, lease)
	}

	if err := client.DeleteActivityLease(ctx, "ibk_test", "m_1", "ibmon-keepalive"); err != nil {
		t.Fatalf("DeleteActivityLease err = %v", err)
	}
	if !deleted.Load() {
		t.Fatalf("expected DELETE activity-lease request to reach server")
	}
}

func TestExecFilesPortsProcessesLogsSnapshotsAndUpdate(t *testing.T) {
	var wroteBytes string
	var snapConfirm string
	client, _ := newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_1/exec":
			_, _ = w.Write([]byte(`{"stdout":"ok\n","stderr":"","exitCode":0,"timedOut":false,"truncated":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_1/files":
			_, _ = w.Write([]byte(`{"path":"/data/app","type":"dir","size":0,"mode":"drwxr-xr-x","modTime":"2026-11-14T00:00:00Z","entries":[{"name":"main.py","type":"file","size":12,"mode":"-rw-r--r--"}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_1/files":
			b := make([]byte, 1024)
			n, _ := r.Body.Read(b)
			wroteBytes = string(b[:n])
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"path":"/data/app/main.py","size":12,"sha256":"abc","mode":"0644","created":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_1/ports":
			_, _ = w.Write([]byte(`{"enabled":true,"ports":[{"id":"p_1","machineId":"m_1","port":8080,"name":"web","url":"https://8080-box.instabox.sh","access":"public","protocol":"http","createdAt":"2026-11-14T00:00:00Z"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_1/ports":
			_, _ = w.Write([]byte(`{"id":"p_2","machineId":"m_1","port":3000,"name":"api","url":"https://3000-box.instabox.sh","access":"private","token":"ibp_tok","tokenUrl":"https://3000-box.instabox.sh?_token=ibp_tok","protocol":"http","createdAt":"2026-11-14T00:00:00Z"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_1/ports/3000":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_1/processes":
			_, _ = w.Write([]byte(`{"machineId":"m_1","state":"running","live":true,"capturedAt":"2026-11-14T00:00:00Z","trigger":"request","truncated":false,"processes":[{"pid":1,"user":"root","command":"init"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_1/logs":
			_, _ = w.Write([]byte(`{"machineId":"m_1","state":"running","console":"boot ok\n","cursor":8,"operations":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/m_1/connection-info":
			_, _ = w.Write([]byte(`{"machineId":"m_1","transport":"ssh","host":"ssh.instabox.sh","port":22,"username":"m_1","capabilities":["pty","exec"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/snapshots":
			_, _ = w.Write([]byte(`{"snapshots":[{"id":"sn_1","name":"pre-upgrade","orgId":"org_1","sourceMachineId":"m_1","sourceSnapshotSeq":1,"rootSizeBytes":100,"createdAt":"2026-11-14T00:00:00Z","status":"ready"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_1/snapshots":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"sn_2","name":"snap-2","orgId":"org_1","sourceMachineId":"m_1","sourceSnapshotSeq":2,"rootSizeBytes":100,"createdAt":"2026-11-14T00:00:00Z","status":"pending"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_1/restore":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_rest","machineId":"m_1","state":"queued"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/snapshots/sn_2":
			snapConfirm = r.Header.Get("X-Confirm-Delete")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/machines/m_1":
			_, _ = w.Write([]byte(`{"id":"m_1","name":"box-updated","state":"running","desiredState":"running","autoSuspendAfterSeconds":0,"maxRunMinutes":-1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ctx := context.Background()
	ex, err := client.ExecMachine(ctx, "ibk_test", "m_1", instabox.ExecInput{Cmd: "echo ok", TimeoutSeconds: 30})
	if err != nil || ex.ExitCode != 0 || ex.Stdout != "ok\n" {
		t.Fatalf("ExecMachine failed: err=%v ex=%+v", err, ex)
	}
	files, err := client.ReadMachineFiles(ctx, "ibk_test", "m_1", "/data/app")
	if err != nil || files.Type != "dir" || len(files.Entries) != 1 {
		t.Fatalf("ReadMachineFiles failed: err=%v files=%+v", err, files)
	}
	fw, err := client.WriteMachineFile(ctx, "ibk_test", "m_1", "/data/app/main.py", "0644", true, []byte("print('hi')\n"))
	if err != nil || !fw.Created || wroteBytes != "print('hi')\n" {
		t.Fatalf("WriteMachineFile failed: err=%v fw=%+v wrote=%q", err, fw, wroteBytes)
	}
	ports, err := client.ListMachinePorts(ctx, "ibk_test", "m_1")
	if err != nil || !ports.Enabled || len(ports.Ports) != 1 {
		t.Fatalf("ListMachinePorts failed: err=%v ports=%+v", err, ports)
	}
	pub, err := client.PublishMachinePort(ctx, "ibk_test", "m_1", instabox.PublishPortInput{Port: 3000, Name: "api", Access: "private"})
	if err != nil || pub.Token != "ibp_tok" {
		t.Fatalf("PublishMachinePort failed: err=%v pub=%+v", err, pub)
	}
	if err := client.UnpublishMachinePort(ctx, "ibk_test", "m_1", 3000); err != nil {
		t.Fatalf("UnpublishMachinePort failed: %v", err)
	}
	procs, err := client.ListMachineProcesses(ctx, "ibk_test", "m_1", false)
	if err != nil || !procs.Live || len(procs.Processes) != 1 {
		t.Fatalf("ListMachineProcesses failed: err=%v procs=%+v", err, procs)
	}
	logs, err := client.GetMachineLogs(ctx, "ibk_test", "m_1", 100, 0)
	if err != nil || logs.Console != "boot ok\n" {
		t.Fatalf("GetMachineLogs failed: err=%v logs=%+v", err, logs)
	}
	conn, err := client.GetMachineConnectionInfo(ctx, "ibk_test", "m_1")
	if err != nil || conn.Host != "ssh.instabox.sh" {
		t.Fatalf("GetMachineConnectionInfo failed: err=%v conn=%+v", err, conn)
	}
	snaps, err := client.ListSnapshots(ctx, "ibk_test")
	if err != nil || len(snaps) != 1 || snaps[0].ID != "sn_1" {
		t.Fatalf("ListSnapshots failed: err=%v snaps=%+v", err, snaps)
	}
	sn, err := client.CreateSnapshot(ctx, "ibk_test", "m_1", instabox.CreateSnapshotInput{Name: "snap-2"})
	if err != nil || sn.ID != "sn_2" {
		t.Fatalf("CreateSnapshot failed: err=%v sn=%+v", err, sn)
	}
	rest, err := client.RestoreMachine(ctx, "ibk_test", "m_1", instabox.RestoreMachineInput{SnapshotID: "sn_1"})
	if err != nil || rest.Operation == nil || rest.Operation.ID != "op_rest" {
		t.Fatalf("RestoreMachine failed: err=%v rest=%+v", err, rest)
	}
	if err := client.DeleteSnapshot(ctx, "ibk_test", "sn_2", "sn_2"); err != nil || snapConfirm != "sn_2" {
		t.Fatalf("DeleteSnapshot failed: err=%v confirm=%q", err, snapConfirm)
	}
	nm := "box-updated"
	upd, err := client.UpdateMachine(ctx, "ibk_test", "m_1", instabox.UpdateMachineInput{Name: &nm})
	if err != nil || upd.Name != "box-updated" {
		t.Fatalf("UpdateMachine failed: err=%v upd=%+v", err, upd)
	}
}
