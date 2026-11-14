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
