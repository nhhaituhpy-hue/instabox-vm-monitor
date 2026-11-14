package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/api"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/monitor"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

type reroute struct {
	u *url.URL
	t http.RoundTripper
}

func (r *reroute) RoundTrip(req *http.Request) (*http.Response, error) {
	c := req.Clone(req.Context())
	u := *c.URL
	u.Scheme = r.u.Scheme
	u.Host = r.u.Host
	c.URL = &u
	return r.t.RoundTrip(c)
}

func setupTestServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	key, err := crypto.LoadOrCreateMasterKey(dir, "")
	if err != nil {
		t.Fatalf("master key: %v", err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), key)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	mockInstabox := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "ibk_valid_secret_key_9999" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_key","message":"bad api key"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		switch r.URL.Path {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"userId":9,"login":"hainh","org":{"id":"org_1","name":"Personal","tier":"free","kind":"personal"},"allowedSpecs":[{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20}]}`))
		case "/v1/templates":
			_, _ = w.Write([]byte(`{"templates":[{"id":"agent-browser","version":"2026-11-01.3","description":"Browser box","arch":"x86_64","creatable":true,"latest":true,"default":true,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16}},{"id":"android","version":"2026-11-02.7","description":"Android 14","arch":"x86_64","creatable":true,"latest":true,"defaultSpec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":24}}]}`))
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"machines":1,"limits":{"maxMachines":1}}`))
		case "/v1/machines":
			if r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["templateId"] == "forbidden-tpl" {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"error":{"code":"template_not_allowed","message":"template requires paid plan"}}`))
					return
				}
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"id":"op_create_2","machineId":"m_2","type":"create","state":"succeeded"}`))
				return
			}
			_, _ = w.Write([]byte(`{"machines":[{"id":"m_1","name":"vm-one","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"runStartedAt":"2026-11-13T10:00:00Z","maxRunMinutes":480,"free":true}],"nextCursor":null}`))
		case "/v1/machines/m_1/usage":
			_, _ = w.Write([]byte(`{"from":"2026-11-01T00:00:00Z","to":"2026-11-13T12:00:00Z","runningSeconds":1200,"vcpuSeconds":4800,"memGiBSeconds":9600,"diskGiBSeconds":24000,"egressBytes":0}`))
		case "/v1/machines/m_1/metrics":
			ts := time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339)
			_, _ = w.Write([]byte(`{"disk":{"measuredAt":"` + ts + `","filesystems":[{"mount":"/data","sizeBytes":21474836480,"usedBytes":2147483648,"availableBytes":19327352832,"usedPct":10,"inodesUsedPct":1,"level":"ok"}]},"metrics":[{"timestamp":"` + ts + `","cpuCount":4,"cpuUsedPct":15.2,"memUsed":1073741824,"memTotal":8589934592,"diskTotal":21474836480,"diskUsed":2147483648}]}`))
		case "/v1/machines/m_1/stop":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_stop_1","machineId":"m_1","state":"succeeded"}`))
		case "/v1/machines/m_1":
			switch r.Method {
			case http.MethodPatch:
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				_, _ = w.Write([]byte(`{"id":"m_1","name":"` + body["name"] + `","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"runStartedAt":"2026-11-13T10:00:00Z","maxRunMinutes":480,"free":true}`))
			case http.MethodDelete:
				if r.Header.Get("X-Confirm-Delete") == "" {
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"error":{"code":"delete_confirmation_required","message":"confirm required"}}`))
					return
				}
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"id":"op_del_1","machineId":"m_1","type":"delete","state":"succeeded"}`))
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockInstabox.Close)

	u, _ := url.Parse(mockInstabox.URL)
	hc := mockInstabox.Client()
	hc.Transport = &reroute{u: u, t: mockInstabox.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	poller := monitor.NewPoller(st, client, 30*time.Second, 7)
	t.Cleanup(poller.Stop)

	h := api.NewRouter(st, client, poller, 24*time.Hour)
	return h, st
}

func TestFirstRunAdminSetupAuthCSRFAndAccountVMFlow(t *testing.T) {
	h, _ := setupTestServer(t)

	// 1. Auth status before setup
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status HTTP %d", rec.Code)
	}
	var status struct {
		NeedsSetup    bool `json:"needsSetup"`
		Authenticated bool `json:"authenticated"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &status)
	if !status.NeedsSetup || status.Authenticated {
		t.Fatalf("unexpected initial auth status: %+v", status)
	}

	// 2. Unauthenticated accounts endpoint must return 401
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/accounts", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated /api/accounts, got %d", rec.Code)
	}

	// 3. First-time admin setup succeeds & returns session cookie + csrfToken
	setupBody := bytes.NewBufferString(`{"username":"admin","password":"SuperSecret!123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/setup", setupBody)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup HTTP %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie after setup")
	}
	var setupResp struct {
		Authenticated bool   `json:"authenticated"`
		CSRFToken     string `json:"csrfToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &setupResp)
	if !setupResp.Authenticated || setupResp.CSRFToken == "" {
		t.Fatalf("unexpected setup response: %+v", setupResp)
	}

	// 4. Second setup attempt must be blocked (409)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBufferString(`{"username":"hacker","password":"Password!123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on repeat setup, got %d", rec.Code)
	}

	// 5. Mutation without CSRF token must fail 403
	req = httptest.NewRequest(http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"alias":"Box 1","apiKey":"ibk_valid_secret_key_9999"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF token, got %d", rec.Code)
	}

	// 6. Add account with valid CSRF + invalid key fails 401
	req = httptest.NewRequest(http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"alias":"Bad","apiKey":"ibk_bad_key"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 400/401 for bad instabox key, got %d: %s", rec.Code, rec.Body.String())
	}

	// 7. Add account with valid key succeeds and does NOT leak raw key
	req = httptest.NewRequest(http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"alias":"Box 1","apiKey":"ibk_valid_secret_key_9999"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add account HTTP %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ibk_valid_secret_key_9999") {
		t.Fatalf("response leaked plaintext instabox api key")
	}
	var accResp struct {
		ID      string `json:"id"`
		KeyMask string `json:"keyMask"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &accResp)
	if accResp.ID == "" || accResp.KeyMask == "" {
		t.Fatalf("unexpected account response: %s", rec.Body.String())
	}

	// 8. List machines returns synced VM and metrics
	req = httptest.NewRequest(http.MethodGet, "/api/machines", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("machines HTTP %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"vm-one"`) {
		t.Fatalf("expected vm-one in /api/machines, got %s", rec.Body.String())
	}

	// 9. Immediate stop action succeeds
	req = httptest.NewRequest(http.MethodPost, "/api/machines/"+accResp.ID+"/m_1/actions/stop", nil)
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stop action HTTP %d: %s", rec.Code, rec.Body.String())
	}

	// 10. HTML root & manifest have security headers + valid JSON content type
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "nonce-") {
		t.Fatalf("expected 200 and CSP nonce on /, got %d csp=%q", rec.Code, rec.Header().Get("Content-Security-Policy"))
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "application/manifest+json") {
		t.Fatalf("expected manifest+json, got %d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestLoginRateLimiterRejectsUntrustedXFFSpoofingAndSetsNoStore(t *testing.T) {
	h, _ := setupTestServer(t)

	// Setup admin first
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBufferString(`{"username":"admin","password":"SuperSecret!123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup HTTP %d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected Cache-Control: no-store on /api response, got %q", rec.Header().Get("Cache-Control"))
	}

	// Send 13 bad login attempts from an untrusted public RemoteAddr with rotating spoofed X-Forwarded-For headers.
	var got429 bool
	for i := range 15 {
		body := bytes.NewBufferString(`{"username":"admin","password":"wrong-password"}`)
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
		r.RemoteAddr = "198.51.100.25:44444"
		r.Header.Set("X-Forwarded-For", "203.0.113."+string(rune('1'+i%9)))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatalf("expected untrusted X-Forwarded-For spoofing to be rate-limited (429)")
	}

	// Behind a trusted reverse proxy (172.18.0.2), spoofed leftmost XFF must be ignored; rightmost untrusted hop (198.51.100.77) is used.
	got429 = false
	for i := range 15 {
		body := bytes.NewBufferString(`{"username":"admin","password":"wrong-password"}`)
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
		r.RemoteAddr = "172.18.0.2:38100"
		r.Header.Set("X-Forwarded-For", "192.0.2."+string(rune('1'+i%9))+", 198.51.100.77")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatalf("expected spoofed leftmost XFF behind trusted proxy to still hit rate-limiter via true client hop")
	}
}

func TestMachineOptionsAndCreateMachineEndpoints(t *testing.T) {
	h, _ := setupTestServer(t)

	// Setup admin
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBufferString(`{"username":"admin","password":"SuperSecret!123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup HTTP %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	var setupResp struct {
		CSRFToken string `json:"csrfToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &setupResp)

	// Unauthenticated /api/accounts/{id}/machine-options -> 401
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/accounts/acc1/machine-options", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated machine-options, got %d", rec.Code)
	}

	// Add account
	req = httptest.NewRequest(http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"alias":"Box 1","apiKey":"ibk_valid_secret_key_9999"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add account HTTP %d", rec.Code)
	}
	var accResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &accResp)

	// Fetch machine-options -> 200 with templates & specs
	req = httptest.NewRequest(http.MethodGet, "/api/accounts/"+accResp.ID+"/machine-options", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("machine-options HTTP %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"agent-browser"`) || !strings.Contains(rec.Body.String(), `"android"`) {
		t.Fatalf("expected templates in machine-options, got %s", rec.Body.String())
	}

	// POST /api/machines without CSRF -> 403
	req = httptest.NewRequest(http.MethodPost, "/api/machines", bytes.NewBufferString(`{"accountId":"`+accResp.ID+`","templateId":"agent-browser"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF, got %d", rec.Code)
	}

	// Invalid machine name -> 400
	req = httptest.NewRequest(http.MethodPost, "/api/machines", bytes.NewBufferString(`{"accountId":"`+accResp.ID+`","name":"Bad Name!","templateId":"agent-browser"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid name, got %d: %s", rec.Code, rec.Body.String())
	}

	// Upstream 403 propagation
	req = httptest.NewRequest(http.MethodPost, "/api/machines", bytes.NewBufferString(`{"accountId":"`+accResp.ID+`","name":"box-two","templateId":"forbidden-tpl"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from upstream template_not_allowed, got %d: %s", rec.Code, rec.Body.String())
	}

	// Valid create -> 202
	req = httptest.NewRequest(http.MethodPost, "/api/machines", bytes.NewBufferString(`{"accountId":"`+accResp.ID+`","name":"box-two","templateId":"android","vcpus":4,"memMiB":8192}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 on create machine, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"op_create_2"`) {
		t.Fatalf("expected operation id in response, got %s", rec.Body.String())
	}
}

func TestRenameAndDeleteMachineEndpoints(t *testing.T) {
	h, _ := setupTestServer(t)

	// Setup admin
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBufferString(`{"username":"admin","password":"SuperSecret!123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	cookies := rec.Result().Cookies()
	var setupResp struct {
		CSRFToken string `json:"csrfToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &setupResp)

	// Add account (populates m_1)
	req = httptest.NewRequest(http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"alias":"Box 1","apiKey":"ibk_valid_secret_key_9999"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var accResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &accResp)

	// Unauth rename -> 401
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1", bytes.NewBufferString(`{"name":"new-box"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated rename, got %d", rec.Code)
	}

	// Missing CSRF on delete -> 403
	req = httptest.NewRequest(http.MethodDelete, "/api/machines/"+accResp.ID+"/m_1", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on delete without CSRF, got %d", rec.Code)
	}

	// Invalid machine name -> 400
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1", bytes.NewBufferString(`{"name":"Invalid Name!"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid rename, got %d: %s", rec.Code, rec.Body.String())
	}

	// Valid rename -> 200 and updates machine name
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1", bytes.NewBufferString(`{"name":"vm-one-renamed"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on rename, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"vm-one-renamed"`) {
		t.Fatalf("expected renamed machine in response, got %s", rec.Body.String())
	}

	// Valid delete -> 200 and removes m_1 from store
	req = httptest.NewRequest(http.MethodDelete, "/api/machines/"+accResp.ID+"/m_1", nil)
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d: %s", rec.Code, rec.Body.String())
	}
	var delBody struct {
		Machines []json.RawMessage `json:"machines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &delBody); err != nil || len(delBody.Machines) != 0 {
		t.Fatalf("expected 0 remaining machines after delete, got err=%v body=%s", err, rec.Body.String())
	}

	// Delete unknown machine -> 404
	req = httptest.NewRequest(http.MethodDelete, "/api/machines/"+accResp.ID+"/m_1", nil)
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on deleting removed machine, got %d: %s", rec.Code, rec.Body.String())
	}
}
