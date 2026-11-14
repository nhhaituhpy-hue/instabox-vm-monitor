package ws

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	OpContinuation byte = 0x0
	OpText         byte = 0x1
	OpBinary       byte = 0x2
	OpClose        byte = 0x8
	OpPing         byte = 0x9
	OpPong         byte = 0xA

	CloseNormalClosure   uint16 = 1000
	CloseGoingAway       uint16 = 1001
	CloseProtocolError   uint16 = 1002
	CloseMessageTooBig   uint16 = 1009
	CloseInternalError   uint16 = 1011
	maxControlPayload           = 125
	defaultMaxMessageLen        = 8 << 20 // 8 MiB
	wsGUID                      = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

type HandshakeError struct {
	StatusCode int
	Body       []byte
}

func (e *HandshakeError) Error() string {
	return fmt.Sprintf("websocket handshake failed: HTTP %d", e.StatusCode)
}

type CloseError struct {
	Code   uint16
	Reason string
}

func (e *CloseError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("websocket closed (%d): %s", e.Code, e.Reason)
	}
	return fmt.Sprintf("websocket closed (%d)", e.Code)
}

type Conn struct {
	netConn     net.Conn
	br          *bufio.Reader
	bw          *bufio.Writer
	isClient    bool
	subprotocol string
	maxMessage  int

	writeMu   sync.Mutex
	closeSent bool
	closeOnce sync.Once
}

func (c *Conn) Subprotocol() string {
	return c.subprotocol
}

func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.netConn.Close()
	})
	return err
}

func sanitizeCloseCode(code uint16) uint16 {
	if code < 1000 || code == 1004 || code == 1005 || code == 1006 || code == 1015 || code >= 5000 {
		return CloseNormalClosure
	}
	return code
}

func (c *Conn) CloseWithCode(code uint16, reason string) error {
	code = sanitizeCloseCode(code)
	payload := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(payload[:2], code)
	copy(payload[2:], reason)
	if len(payload) > maxControlPayload {
		payload = payload[:maxControlPayload]
	}
	_ = c.WriteControl(OpClose, payload)
	return c.Close()
}

func Upgrade(w http.ResponseWriter, r *http.Request, protocols []string) (*Conn, error) {
	if r.Method != http.MethodGet {
		return nil, errors.New("websocket upgrade requires GET")
	}
	if !headerContainsToken(r.Header, "Connection", "upgrade") || !headerContainsToken(r.Header, "Upgrade", "websocket") {
		return nil, errors.New("missing websocket upgrade headers")
	}
	if strings.TrimSpace(r.Header.Get("Sec-WebSocket-Version")) != "13" {
		return nil, errors.New("unsupported Sec-WebSocket-Version")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}
	if rawKey, err := base64.StdEncoding.DecodeString(key); err != nil || len(rawKey) != 16 {
		return nil, errors.New("invalid Sec-WebSocket-Key")
	}

	selectedProto := negotiateSubprotocol(r.Header.Values("Sec-WebSocket-Protocol"), protocols)

	rc := http.NewResponseController(w)
	netConn, rw, err := rc.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack connection: %w", err)
	}
	_ = netConn.SetDeadline(time.Time{})

	var resp strings.Builder
	resp.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	resp.WriteString("Upgrade: websocket\r\n")
	resp.WriteString("Connection: Upgrade\r\n")
	resp.WriteString("Sec-WebSocket-Accept: ")
	resp.WriteString(computeAcceptKey(key))
	resp.WriteString("\r\n")
	if selectedProto != "" {
		resp.WriteString("Sec-WebSocket-Protocol: ")
		resp.WriteString(selectedProto)
		resp.WriteString("\r\n")
	}
	resp.WriteString("\r\n")

	if _, err := rw.WriteString(resp.String()); err != nil {
		_ = netConn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = netConn.Close()
		return nil, err
	}

	return &Conn{
		netConn:     netConn,
		br:          rw.Reader,
		bw:          rw.Writer,
		isClient:    false,
		subprotocol: selectedProto,
		maxMessage:  defaultMaxMessageLen,
	}, nil
}

func DialContext(ctx context.Context, rawURL string, headers http.Header, protocols []string, tlsCfg *tls.Config) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid websocket url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http":
		scheme = "ws"
	case "https":
		scheme = "wss"
	case "ws", "wss":
	default:
		return nil, fmt.Errorf("unsupported websocket scheme %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("missing websocket host")
	}
	port := u.Port()
	if port == "" {
		if scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}
	addr := net.JoinHostPort(host, port)

	dialer := &net.Dialer{Timeout: 20 * time.Second}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	var netConn net.Conn = rawConn
	if scheme == "wss" {
		var cfg *tls.Config
		if tlsCfg != nil {
			cfg = tlsCfg.Clone()
		} else {
			cfg = &tls.Config{}
		}
		if cfg.ServerName == "" {
			cfg.ServerName = host
		}
		if cfg.MinVersion == 0 {
			cfg.MinVersion = tls.VersionTLS12
		}
		// Force HTTP/1.1 ALPN so upstream does not negotiate h2 over TLS.
		cfg.NextProtos = []string{"http/1.1"}
		tlsConn := tls.Client(rawConn, cfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}
		netConn = tlsConn
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = netConn.SetDeadline(deadline)
	}

	keyBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		_ = netConn.Close()
		return nil, err
	}
	wsKey := base64.StdEncoding.EncodeToString(keyBytes)

	reqURI := u.EscapedPath()
	if reqURI == "" {
		reqURI = "/"
	}
	if u.RawQuery != "" {
		reqURI += "?" + u.RawQuery
	}

	hostHeader := u.Host
	if hostHeader == "" {
		hostHeader = host
	}

	var reqBuf strings.Builder
	reqBuf.WriteString("GET ")
	reqBuf.WriteString(reqURI)
	reqBuf.WriteString(" HTTP/1.1\r\n")
	reqBuf.WriteString("Host: ")
	reqBuf.WriteString(hostHeader)
	reqBuf.WriteString("\r\n")
	reqBuf.WriteString("Upgrade: websocket\r\n")
	reqBuf.WriteString("Connection: Upgrade\r\n")
	reqBuf.WriteString("Sec-WebSocket-Version: 13\r\n")
	reqBuf.WriteString("Sec-WebSocket-Key: ")
	reqBuf.WriteString(wsKey)
	reqBuf.WriteString("\r\n")
	if len(protocols) > 0 {
		reqBuf.WriteString("Sec-WebSocket-Protocol: ")
		reqBuf.WriteString(strings.Join(protocols, ", "))
		reqBuf.WriteString("\r\n")
	}
	for k, vals := range headers {
		kl := strings.ToLower(k)
		if kl == "host" || kl == "upgrade" || kl == "connection" || kl == "sec-websocket-version" || kl == "sec-websocket-key" || kl == "sec-websocket-protocol" {
			continue
		}
		for _, v := range vals {
			cleanV := strings.NewReplacer("\r", "", "\n", "").Replace(v)
			reqBuf.WriteString(k)
			reqBuf.WriteString(": ")
			reqBuf.WriteString(cleanV)
			reqBuf.WriteString("\r\n")
		}
	}
	reqBuf.WriteString("\r\n")

	if _, err := io.WriteString(netConn, reqBuf.String()); err != nil {
		_ = netConn.Close()
		return nil, err
	}

	br := bufio.NewReader(netConn)
	fakeReq := &http.Request{Method: http.MethodGet}
	resp, err := http.ReadResponse(br, fakeReq)
	if err != nil {
		_ = netConn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		_ = netConn.Close()
		return nil, &HandshakeError{StatusCode: resp.StatusCode, Body: body}
	}
	_ = resp.Body.Close()

	if !headerContainsToken(resp.Header, "Connection", "upgrade") || !headerContainsToken(resp.Header, "Upgrade", "websocket") {
		_ = netConn.Close()
		return nil, errors.New("invalid websocket upgrade response headers")
	}
	if strings.TrimSpace(resp.Header.Get("Sec-WebSocket-Accept")) != computeAcceptKey(wsKey) {
		_ = netConn.Close()
		return nil, errors.New("invalid Sec-WebSocket-Accept from server")
	}

	// Clear handshake deadline for long-lived interactive streaming.
	_ = netConn.SetDeadline(time.Time{})

	return &Conn{
		netConn:     netConn,
		br:          br,
		bw:          bufio.NewWriter(netConn),
		isClient:    true,
		subprotocol: strings.TrimSpace(resp.Header.Get("Sec-WebSocket-Protocol")),
		maxMessage:  defaultMaxMessageLen,
	}, nil
}

func (c *Conn) ReadMessage() (byte, []byte, error) {
	var msgOp byte
	var msgBuf []byte
	inFragment := false

	for {
		fin, opcode, payload, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}

		// Handle control frames (allowed between fragments per RFC 6455).
		if opcode >= 0x8 {
			if !fin || len(payload) > maxControlPayload {
				_ = c.CloseWithCode(CloseProtocolError, "invalid control frame")
				return 0, nil, errors.New("invalid control frame")
			}
			switch opcode {
			case OpPing:
				_ = c.WriteControl(OpPong, payload)
			case OpPong:
				// Keepalive acknowledged.
			case OpClose:
				code := CloseNormalClosure
				reason := ""
				if len(payload) >= 2 {
					code = binary.BigEndian.Uint16(payload[:2])
					reason = string(payload[2:])
				}
				_ = c.WriteControl(OpClose, payload)
				return 0, nil, &CloseError{Code: code, Reason: reason}
			default:
				_ = c.CloseWithCode(CloseProtocolError, "unknown control opcode")
				return 0, nil, fmt.Errorf("unknown control opcode 0x%x", opcode)
			}
			continue
		}

		switch opcode {
		case OpText, OpBinary:
			if inFragment {
				_ = c.CloseWithCode(CloseProtocolError, "unexpected data frame in fragment")
				return 0, nil, errors.New("unexpected data frame during fragmented message")
			}
			if fin {
				return opcode, payload, nil
			}
			msgOp = opcode
			msgBuf = append(msgBuf, payload...)
			inFragment = true
		case OpContinuation:
			if !inFragment {
				_ = c.CloseWithCode(CloseProtocolError, "unexpected continuation frame")
				return 0, nil, errors.New("unexpected continuation frame")
			}
			if len(msgBuf)+len(payload) > c.maxMessage {
				_ = c.CloseWithCode(CloseMessageTooBig, "message too large")
				return 0, nil, errors.New("websocket message too large")
			}
			msgBuf = append(msgBuf, payload...)
			if fin {
				return msgOp, msgBuf, nil
			}
		default:
			_ = c.CloseWithCode(CloseProtocolError, "unknown data opcode")
			return 0, nil, fmt.Errorf("unknown opcode 0x%x", opcode)
		}
	}
}

func (c *Conn) readFrame() (bool, byte, []byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
		return false, 0, nil, err
	}
	fin := (hdr[0] & 0x80) != 0
	if (hdr[0] & 0x70) != 0 {
		_ = c.CloseWithCode(CloseProtocolError, "unsupported RSV bits")
		return false, 0, nil, errors.New("unsupported RSV bits")
	}
	opcode := hdr[0] & 0x0F
	masked := (hdr[1] & 0x80) != 0
	length := uint64(hdr[1] & 0x7F)

	switch length {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext[:])
	}

	if length > uint64(c.maxMessage) {
		_ = c.CloseWithCode(CloseMessageTooBig, "frame too large")
		return false, 0, nil, errors.New("websocket frame too large")
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, maskKey[:]); err != nil {
			return false, 0, nil, err
		}
	}

	// RFC 6455: server expects masked frames from clients; client expects unmasked frames from server.
	if !c.isClient && !masked {
		_ = c.CloseWithCode(CloseProtocolError, "client frame must be masked")
		return false, 0, nil, errors.New("unmasked client frame")
	}

	payload := make([]byte, int(length))
	if length > 0 {
		if _, err := io.ReadFull(c.br, payload); err != nil {
			return false, 0, nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}
	}
	return fin, opcode, payload, nil
}

func (c *Conn) WriteMessage(opcode byte, payload []byte) error {
	if opcode != OpText && opcode != OpBinary {
		return fmt.Errorf("invalid data opcode 0x%x", opcode)
	}
	return c.writeFrame(opcode, payload)
}

func (c *Conn) WriteControl(opcode byte, payload []byte) error {
	if len(payload) > maxControlPayload {
		payload = payload[:maxControlPayload]
	}
	return c.writeFrame(opcode, payload)
}

func (c *Conn) Ping(payload []byte) error {
	return c.WriteControl(OpPing, payload)
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.closeSent {
		return net.ErrClosed
	}
	if opcode == OpClose {
		c.closeSent = true
	}

	var header [14]byte
	header[0] = 0x80 | (opcode & 0x0F)
	n := len(payload)
	idx := 2
	var maskBit byte
	if c.isClient {
		maskBit = 0x80
	}

	switch {
	case n <= 125:
		header[1] = maskBit | byte(n)
	case n <= 65535:
		header[1] = maskBit | 126
		binary.BigEndian.PutUint16(header[2:4], uint16(n))
		idx = 4
	default:
		header[1] = maskBit | 127
		binary.BigEndian.PutUint64(header[2:10], uint64(n))
		idx = 10
	}

	var maskKey [4]byte
	if c.isClient {
		if _, err := io.ReadFull(rand.Reader, maskKey[:]); err != nil {
			return err
		}
		copy(header[idx:idx+4], maskKey[:])
		idx += 4
	}

	if _, err := c.bw.Write(header[:idx]); err != nil {
		return err
	}
	if n > 0 {
		if c.isClient {
			masked := make([]byte, n)
			for i, b := range payload {
				masked[i] = b ^ maskKey[i%4]
			}
			if _, err := c.bw.Write(masked); err != nil {
				return err
			}
		} else {
			if _, err := c.bw.Write(payload); err != nil {
				return err
			}
		}
	}
	return c.bw.Flush()
}

// Relay pumps text/binary messages between client and upstream WebSocket connections
// until either side disconnects or ctx is cancelled, sending periodic keepalive pings.
func Relay(ctx context.Context, client, upstream *Conn, pingInterval time.Duration) {
	if pingInterval <= 0 {
		pingInterval = 25 * time.Second
	}

	done := make(chan struct{})
	var closeOnce sync.Once
	shutdown := func() {
		closeOnce.Do(func() {
			close(done)
			_ = client.Close()
			_ = upstream.Close()
		})
	}
	defer shutdown()

	errCh := make(chan error, 2)

	// client -> upstream
	go func() {
		for {
			op, data, err := client.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := upstream.WriteMessage(op, data); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// upstream -> client
	go func() {
		for {
			op, data, err := upstream.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := client.WriteMessage(op, data); err != nil {
				errCh <- err
				return
			}
		}
	}()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = client.CloseWithCode(CloseGoingAway, "server shutdown")
			_ = upstream.CloseWithCode(CloseGoingAway, "client disconnected")
			return
		case err := <-errCh:
			var ce *CloseError
			if errors.As(err, &ce) {
				_ = client.CloseWithCode(ce.Code, ce.Reason)
				_ = upstream.CloseWithCode(ce.Code, ce.Reason)
			} else {
				_ = client.CloseWithCode(CloseNormalClosure, "")
				_ = upstream.CloseWithCode(CloseNormalClosure, "")
			}
			return
		case <-ticker.C:
			ts := []byte(strconv.FormatInt(time.Now().Unix(), 10))
			if err := upstream.Ping(ts); err != nil {
				return
			}
			if err := client.Ping(ts); err != nil {
				return
			}
		}
	}
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte(wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func headerContainsToken(h http.Header, name, token string) bool {
	want := strings.ToLower(token)
	for _, v := range h.Values(name) {
		for _, part := range strings.Split(v, ",") {
			if strings.ToLower(strings.TrimSpace(part)) == want {
				return true
			}
		}
	}
	return false
}

func negotiateSubprotocol(clientHeaders []string, supported []string) string {
	if len(supported) == 0 {
		return ""
	}
	allowed := make(map[string]bool, len(supported))
	for _, s := range supported {
		allowed[s] = true
	}
	for _, raw := range clientHeaders {
		for _, part := range strings.Split(raw, ",") {
			cand := strings.TrimSpace(part)
			if allowed[cand] {
				return cand
			}
		}
	}
	return ""
}
