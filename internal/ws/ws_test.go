package ws

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebSocketRoundTripPingPongAndRelay(t *testing.T) {
	// Upstream mock WebSocket echo + terminal control server
	upstreamSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "ibk_secret" {
			http.Error(w, `{"error":{"code":"unauthorized","message":"bad key"}}`, http.StatusUnauthorized)
			return
		}
		c, err := Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upstream upgrade: %v", err)
			return
		}
		defer c.Close()

		_ = c.WriteMessage(OpText, []byte(`{"type":"session","session":"main","persistent":true}`))
		for {
			op, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if err := c.WriteMessage(op, data); err != nil {
				return
			}
		}
	}))
	defer upstreamSrv.Close()

	tlsCfg := upstreamSrv.Client().Transport.(*http.Transport).TLSClientConfig

	// Proxy server mimicking handleMachineTerminal
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientWS, err := Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("proxy upgrade: %v", err)
			return
		}
		defer clientWS.Close()

		headers := http.Header{}
		headers.Set("X-API-Key", "ibk_secret")
		upstreamWS, err := DialContext(r.Context(), upstreamSrv.URL+"/v1/machines/m_1/terminal", headers, nil, tlsCfg)
		if err != nil {
			t.Errorf("proxy dial upstream: %v", err)
			return
		}
		defer upstreamWS.Close()

		Relay(r.Context(), clientWS, upstreamWS, 50*time.Millisecond)
	}))
	defer proxySrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := DialContext(ctx, proxySrv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer client.Close()

	op, msg, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read session msg: %v", err)
	}
	if op != OpText || !strings.Contains(string(msg), `"session":"main"`) {
		t.Fatalf("unexpected initial frame: op=%d msg=%q", op, string(msg))
	}

	// Send binary terminal bytes & expect echo
	payload := bytes.Repeat([]byte("ls -la\r"), 50)
	if err := client.WriteMessage(OpBinary, payload); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	op, echoed, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read binary echo: %v", err)
	}
	if op != OpBinary || !bytes.Equal(echoed, payload) {
		t.Fatalf("binary echo mismatch: op=%d len=%d", op, len(echoed))
	}

	// Send control resize JSON
	ctrl := []byte(`{"type":"resize","cols":120,"rows":36}`)
	if err := client.WriteMessage(OpText, ctrl); err != nil {
		t.Fatalf("write resize: %v", err)
	}
	op, ctrlEcho, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read resize echo: %v", err)
	}
	if op != OpText || string(ctrlEcho) != string(ctrl) {
		t.Fatalf("ctrl echo mismatch: op=%d msg=%q", op, string(ctrlEcho))
	}

	// Verify RFC 6455 Section 1.3 Sec-WebSocket-Accept test vector.
	if got := computeAcceptKey("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("RFC 6455 Sec-WebSocket-Accept vector mismatch: got %q", got)
	}

	// Verify HandshakeError when upstream rejects with 409
	rejectSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"machine_not_running","message":"machine is stopped"}}`))
	}))
	defer rejectSrv.Close()

	_, err = DialContext(ctx, rejectSrv.URL, nil, nil, rejectSrv.Client().Transport.(*http.Transport).TLSClientConfig)
	var he *HandshakeError
	if !errors.As(err, &he) || he.StatusCode != http.StatusConflict {
		t.Fatalf("expected HandshakeError 409, got %v", err)
	}
}
