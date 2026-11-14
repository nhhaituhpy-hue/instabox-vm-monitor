package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/mcp"
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

func rpcCall(t *testing.T, h http.Handler, sessionID string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("MCP-Session-Id", sessionID)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMCPStreamableHTTPHandshakeExecAndGatedDelete(t *testing.T) {
	dir := t.TempDir()
	key, _ := crypto.LoadOrCreateMasterKey(dir, "")
	st, _ := store.Open(filepath.Join(dir, "mcp.db"), key)
	t.Cleanup(func() { _ = st.Close() })

	var deletedUpstream atomic.Bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/m_1/exec":
			_, _ = w.Write([]byte(`{"stdout":"v1.2.3\n","stderr":"","exitCode":0,"timedOut":false,"truncated":false}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/machines/m_1/files":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"path":"/data/run.sh","size":10,"sha256":"abc","mode":"0755","created":true}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/m_1":
			if r.Header.Get("X-Confirm-Delete") == "" {
				w.WriteHeader(http.StatusConflict)
				return
			}
			deletedUpstream.Store(true)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"op_del","machineId":"m_1","state":"succeeded"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	hc := srv.Client()
	hc.Transport = &reroute{u: u, t: srv.Client().Transport}
	client, _ := instabox.NewClient("https://api.instabox.sh", hc)

	acc, _ := st.CreateAccount(context.Background(), store.AccountInput{Alias: "Box 1", APIKey: "ibk_test_key_1234", OrgTier: "free"})
	_ = st.SyncAccountMachines(context.Background(), acc.ID, []store.MachineRecord{
		{AccountID: acc.ID, MachineID: "m_1", Name: "hermes-box", State: "running", DesiredState: "running"},
	})

	poller := monitor.NewPoller(st, client, 30*time.Second, 7)
	defer poller.Stop()

	handler := mcp.NewHandler(st, poller)

	// 1. GET /mcp returns 405 Method Not Allowed per Streamable HTTP spec
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET /mcp, got %d", rec.Code)
	}

	// 2. initialize
	rec = rpcCall(t, handler, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"hermes","version":"1.0"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize HTTP %d: %s", rec.Code, rec.Body.String())
	}
	sid := rec.Header().Get("MCP-Session-Id")
	if sid == "" || !strings.Contains(rec.Body.String(), `"protocolVersion":"2025-03-26"`) {
		t.Fatalf("unexpected initialize response: sid=%q body=%s", sid, rec.Body.String())
	}

	// 3. notifications/initialized -> 202 Accepted
	rec = rpcCall(t, handler, sid, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 on notification, got %d", rec.Code)
	}

	// 4. tools/list
	rec = rpcCall(t, handler, sid, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"exec_command"`) || !strings.Contains(rec.Body.String(), `"delete_machine"`) {
		t.Fatalf("unexpected tools/list: %d %s", rec.Code, rec.Body.String())
	}

	// 5. exec_command (resolves account automatically from unique machineId)
	rec = rpcCall(t, handler, sid, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"exec_command","arguments":{"machineId":"m_1","cmd":"node -v"}}}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `v1.2.3`) {
		t.Fatalf("exec_command failed: %d %s", rec.Code, rec.Body.String())
	}

	// 6. write_file
	rec = rpcCall(t, handler, sid, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"machineId":"m_1","path":"/data/run.sh","mode":"0755","parents":true,"content":"echo hi\n"}}}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `/data/run.sh`) {
		t.Fatalf("write_file failed: %d %s", rec.Code, rec.Body.String())
	}

	// 7. delete_machine WITHOUT confirmationToken MUST NOT delete; returns challenge token
	rec = rpcCall(t, handler, sid, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"delete_machine","arguments":{"machineId":"m_1"}}}`)
	if rec.Code != http.StatusOK || deletedUpstream.Load() || !strings.Contains(rec.Body.String(), `requiresUserConfirmation`) {
		t.Fatalf("expected delete_machine without token to require confirmation, got deleted=%v body=%s", deletedUpstream.Load(), rec.Body.String())
	}
	var toolEnv struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &toolEnv)
	var challenge struct {
		ConfirmationToken string `json:"confirmationToken"`
	}
	_ = json.Unmarshal([]byte(toolEnv.Result.Content[0].Text), &challenge)
	if challenge.ConfirmationToken == "" {
		t.Fatalf("expected confirmationToken in tool response, got %s", rec.Body.String())
	}

	// 8. delete_machine WITH mismatched confirmName fails without consuming/deleting
	rec = rpcCall(t, handler, sid, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"delete_machine","arguments":{"machineId":"m_1","confirmationToken":"`+challenge.ConfirmationToken+`","confirmName":"wrong-name"}}}`)
	if deletedUpstream.Load() || !strings.Contains(rec.Body.String(), `"isError":true`) {
		t.Fatalf("expected wrong confirmName to fail without deleting, got %s", rec.Body.String())
	}

	// 9. delete_machine WITH valid confirmationToken & confirmName deletes machine
	rec = rpcCall(t, handler, sid, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"delete_machine","arguments":{"machineId":"m_1","confirmationToken":"`+challenge.ConfirmationToken+`","confirmName":"hermes-box"}}}`)
	if rec.Code != http.StatusOK || !deletedUpstream.Load() {
		t.Fatalf("expected confirmed delete_machine to succeed, got %d %s", rec.Code, rec.Body.String())
	}

	// 10. DELETE /mcp terminates session -> 204
	delReq := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	delReq.Header.Set("MCP-Session-Id", sid)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, delReq)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on DELETE /mcp, got %d", rec.Code)
	}
}
