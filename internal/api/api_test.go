package api_test

import (
	"bytes"
	"context"
	"crypto/tls"
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
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/ws"
)

type reroute struct {
	u *url.URL
	t http.RoundTripper
}

func (r *reroute) TerminalDialTarget() (string, *tls.Config) {
	if tr, ok := r.t.(*http.Transport); ok {
		return r.u.String(), tr.TLSClientConfig
	}
	return r.u.String(), nil
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
		case "/v1/machines/m_1/terminal":
			if r.URL.Query().Get("wake") == "0" {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":{"code":"machine_not_running","message":"machine is suspended"}}`))
				return
			}
			up, err := ws.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer up.Close()
			sess := r.URL.Query().Get("session")
			if sess == "" {
				sess = "main"
			}
			_ = up.WriteMessage(ws.OpText, []byte(`{"type":"session","session":"`+sess+`","persistent":true}`))
			for {
				op, data, err := up.ReadMessage()
				if err != nil {
					return
				}
				if err := up.WriteMessage(op, data); err != nil {
					return
				}
			}
		case "/v1/machines/m_1/activity-leases/ibmon-keepalive":
			if r.Method == http.MethodPut {
				_, _ = w.Write([]byte(`{"id":"ibmon-keepalive","machineId":"m_1","expiresAt":"2026-11-14T04:00:00Z"}`))
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		case "/v1/machines/m_1":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(`{"id":"m_1","name":"vm-one","state":"running","desiredState":"running","spec":{"vcpus":4,"memMiB":8192,"persistentDiskGiB":20,"rootDiskGiB":16},"storage":{"sizeGiB":20,"mountPath":"/data","persistent":true,"synced":true},"runtime":{"ready":true},"runStartedAt":"2026-11-13T10:00:00Z","maxRunMinutes":480,"free":true}`))
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

func TestMachineKeepaliveEndpoint(t *testing.T) {
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

	// Unauthenticated -> 401
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1/keepalive", bytes.NewBufferString(`{"enabled":true}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated keepalive toggle, got %d", rec.Code)
	}

	// Missing CSRF -> 403
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1/keepalive", bytes.NewBufferString(`{"enabled":true}`))
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF, got %d", rec.Code)
	}

	// Missing enabled boolean -> 400
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1/keepalive", bytes.NewBufferString(`{}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when enabled is missing, got %d: %s", rec.Code, rec.Body.String())
	}

	// Wrong type for enabled -> 400
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1/keepalive", bytes.NewBufferString(`{"enabled":"true"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when enabled is not boolean, got %d: %s", rec.Code, rec.Body.String())
	}

	// Enable keepalive -> 200 with keepaliveEnabled: true
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1/keepalive", bytes.NewBufferString(`{"enabled":true}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"keepaliveEnabled":true`) {
		t.Fatalf("expected 200 keepaliveEnabled=true, got %d: %s", rec.Code, rec.Body.String())
	}

	// Disable keepalive -> 200 with keepaliveEnabled: false
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_1/keepalive", bytes.NewBufferString(`{"enabled":false}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"keepaliveEnabled":false`) {
		t.Fatalf("expected 200 keepaliveEnabled=false, got %d: %s", rec.Code, rec.Body.String())
	}

	// Unknown machine -> 404
	req = httptest.NewRequest(http.MethodPatch, "/api/machines/"+accResp.ID+"/m_missing/keepalive", bytes.NewBufferString(`{"enabled":true}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unknown machine, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeysMCPAuthAndGatedDeleteOverHTTP(t *testing.T) {
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

	// 1. Unauthenticated /mcp and /api/api-keys -> 401
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated /mcp, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/api-keys", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated /api/api-keys, got %d", rec.Code)
	}

	// 2. Create Service API Key via session + CSRF
	req = httptest.NewRequest(http.MethodPost, "/api/api-keys", bytes.NewBufferString(`{"name":"Hermes"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var keyResp struct {
		Token  string `json:"token"`
		APIKey struct {
			ID string `json:"id"`
		} `json:"apiKey"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &keyResp)
	if !strings.HasPrefix(keyResp.Token, "ibmon_") || keyResp.APIKey.ID == "" {
		t.Fatalf("unexpected api key create response: %s", rec.Body.String())
	}

	// 3. Bearer token works on /mcp initialize + tools/list
	req = httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`))
	req.Header.Set("Authorization", "Bearer "+keyResp.Token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"instabox-vm-monitor"`) {
		t.Fatalf("MCP with Bearer token failed: %d %s", rec.Code, rec.Body.String())
	}

	// 4. Bearer token CANNOT manage /api/api-keys (session-only admin)
	req = httptest.NewRequest(http.MethodGet, "/api/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+keyResp.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected API key blocked from /api/api-keys, got %d", rec.Code)
	}

	// 5. Bearer token CAN execute actions without CSRF (and does NOT panic on nil session!)
	req = httptest.NewRequest(http.MethodPost, "/api/machines/"+accResp.ID+"/m_1/actions/stop", nil)
	req.Header.Set("Authorization", "Bearer "+keyResp.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected Bearer action stop 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Bearer DELETE /api/machines without confirmation token returns 409 challenge instead of deleting!
	req = httptest.NewRequest(http.MethodDelete, "/api/machines/"+accResp.ID+"/m_1", nil)
	req.Header.Set("Authorization", "Bearer "+keyResp.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"requiresUserConfirmation":true`) {
		t.Fatalf("expected 409 confirmation challenge for Bearer delete, got %d: %s", rec.Code, rec.Body.String())
	}
	var ch struct {
		ConfirmationToken string `json:"confirmationToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ch)

	// 7. Bearer DELETE WITH confirmation token succeeds
	req = httptest.NewRequest(http.MethodDelete, "/api/machines/"+accResp.ID+"/m_1", nil)
	req.Header.Set("Authorization", "Bearer "+keyResp.Token)
	req.Header.Set("X-Confirm-Delete-Token", ch.ConfirmationToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on confirmed Bearer delete, got %d: %s", rec.Code, rec.Body.String())
	}

	// 8. Revoking API key via session immediately rejects subsequent /mcp calls
	req = httptest.NewRequest(http.MethodDelete, "/api/api-keys/"+keyResp.APIKey.ID, nil)
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 revoking api key, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+keyResp.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after key revocation, got %d", rec.Code)
	}
}

func TestMachineTerminalWebSocketProxyAndOriginChecks(t *testing.T) {
	h, _ := setupTestServer(t)

	// Setup admin session
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBufferString(`{"username":"admin","password":"SuperSecret!123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	cookies := rec.Result().Cookies()
	var setupResp struct {
		CSRFToken string `json:"csrfToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &setupResp)

	// Create API key (must be rejected on browser terminal WS)
	req = httptest.NewRequest(http.MethodPost, "/api/api-keys", bytes.NewBufferString(`{"name":"Hermes"}`))
	req.AddCookie(cookies[0])
	req.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var keyResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &keyResp)

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

	srv := httptest.NewServer(h)
	defer srv.Close()

	termURL := srv.URL + "/api/machines/" + accResp.ID + "/m_1/terminal?cols=100&rows=30&session=dev-1"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Unauthenticated -> 401
	_, err := ws.DialContext(ctx, termURL, http.Header{"Origin": {srv.URL}}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected HTTP 401 for unauthenticated WS, got %v", err)
	}

	// 2. Bearer API key -> 403 session_required
	_, err = ws.DialContext(ctx, termURL, http.Header{
		"Origin":        {srv.URL},
		"Authorization": {"Bearer " + keyResp.Token},
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected HTTP 403 for API key on WS terminal, got %v", err)
	}

	// 3. Cross-origin Origin header -> 403 origin_forbidden
	cookieHeader := cookies[0].Name + "=" + cookies[0].Value
	_, err = ws.DialContext(ctx, termURL, http.Header{
		"Origin": {"https://evil.example"},
		"Cookie": {cookieHeader},
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected HTTP 403 for cross-origin WS, got %v", err)
	}

	// 4. Invalid session name -> 400
	badURL := srv.URL + "/api/machines/" + accResp.ID + "/m_1/terminal?session=BAD_SESSION!"
	_, err = ws.DialContext(ctx, badURL, http.Header{
		"Origin": {srv.URL},
		"Cookie": {cookieHeader},
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected HTTP 400 for invalid session, got %v", err)
	}

	// 5. Valid same-origin session WS -> relays session banner, binary bytes, and resize control
	conn, err := ws.DialContext(ctx, termURL, http.Header{
		"Origin": {srv.URL},
		"Cookie": {cookieHeader},
	}, nil, nil)
	if err != nil {
		t.Fatalf("dial terminal WS: %v", err)
	}
	defer conn.Close()

	op, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read initial session msg: %v", err)
	}
	if op != ws.OpText || !strings.Contains(string(msg), `"session":"dev-1"`) {
		t.Fatalf("expected session=dev-1 banner, got op=%d msg=%q", op, string(msg))
	}

	if err := conn.WriteMessage(ws.OpBinary, []byte("uname -a\r")); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	op, echoed, err := conn.ReadMessage()
	if err != nil || op != ws.OpBinary || string(echoed) != "uname -a\r" {
		t.Fatalf("binary echo mismatch: op=%d echoed=%q err=%v", op, string(echoed), err)
	}

	if err := conn.WriteMessage(ws.OpText, []byte(`{"type":"resize","cols":132,"rows":40}`)); err != nil {
		t.Fatalf("write resize: %v", err)
	}
	op, ctrlEcho, err := conn.ReadMessage()
	if err != nil || op != ws.OpText || !strings.Contains(string(ctrlEcho), `"cols":132`) {
		t.Fatalf("resize echo mismatch: op=%d ctrl=%q err=%v", op, string(ctrlEcho), err)
	}

	// 6. Reverse-proxy X-Forwarded-Proto: wss with https Origin is accepted as same-origin
	proxyConn, err := ws.DialContext(ctx, termURL, http.Header{
		"Origin":            {"https://instabox.hainh.io.vn"},
		"X-Forwarded-Proto": {"wss"},
		"X-Forwarded-Host":  {"instabox.hainh.io.vn"},
		"Cookie":            {cookieHeader},
	}, nil, nil)
	if err != nil {
		t.Fatalf("dial terminal WS behind wss proxy: %v", err)
	}
	_ = proxyConn.Close()

	// 7. Upstream 409 surfaces as structured JSON error frame over WS
	errConn, err := ws.DialContext(ctx, termURL+"&wake=0", http.Header{
		"Origin": {srv.URL},
		"Cookie": {cookieHeader},
	}, nil, nil)
	if err != nil {
		t.Fatalf("dial error case WS: %v", err)
	}
	defer errConn.Close()
	op, errFrame, err := errConn.ReadMessage()
	if err != nil || op != ws.OpText || !strings.Contains(string(errFrame), `"machine_not_running"`) {
		t.Fatalf("expected machine_not_running error frame, got op=%d msg=%q err=%v", op, string(errFrame), err)
	}
}

func TestCreateMachineLifetimeValidationAndPassthrough(t *testing.T) {
	h, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBufferString(`{"username":"«redacted-vault-secret»","password":"SuperSecret!123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	cookies := rec.Result().Cookies()
	var setupResp struct {
		CSRFToken string `json:"csrfToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &setupResp)

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

	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/machines", bytes.NewBufferString(body))
		r.AddCookie(cookies[0])
		r.Header.Set("X-CSRF-Token", setupResp.CSRFToken)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr
	}

	// autoSuspend not a whole minute -> 400 (new upstream rule: whole minutes 5-30 or 0)
	rr := post(`{"accountId":"` + accResp.ID + `","templateId":"android","autoSuspendAfterSeconds":450}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-minute autoSuspend, got %d: %s", rr.Code, rr.Body.String())
	}
	// autoSuspend outside 5-30 -> 400
	rr = post(`{"accountId":"` + accResp.ID + `","templateId":"android","autoSuspendAfterSeconds":3600}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range autoSuspend, got %d: %s", rr.Code, rr.Body.String())
	}
	// deleteAfterSeconds out of range -> 400
	rr = post(`{"accountId":"` + accResp.ID + `","templateId":"android","deleteAfterSeconds":30}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for deleteAfterSeconds < 60, got %d: %s", rr.Code, rr.Body.String())
	}
	// valid: whole minute + valid deleteAfter/archiveSeconds -> 202
	rr = post(`{"accountId":"` + accResp.ID + `","templateId":"android","autoSuspendAfterSeconds":600,"deleteAfterSeconds":3600,"archiveSeconds":7200}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for valid lifetime create, got %d: %s", rr.Code, rr.Body.String())
	}
	// autoSuspend=0 (always-on) is a valid wire value upstream for paid plans
	rr = post(`{"accountId":"` + accResp.ID + `","templateId":"android","autoSuspendAfterSeconds":0}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for autoSuspend=0, got %d: %s", rr.Code, rr.Body.String())
	}
}
