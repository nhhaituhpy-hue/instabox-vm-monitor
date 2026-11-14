package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/monitor"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/web"
)

const (
	sessionCookieHTTPS = "__Host-ibmon_session"
	sessionCookiePlain = "ibmon_session"
)

type Server struct {
	store      *store.Store
	client     *instabox.Client
	poller     *monitor.Poller
	sessionTTL time.Duration
	staticFS   fs.FS
	indexTmpl  string

	loginLimiter *rateLimiter
}

func NewRouter(st *store.Store, client *instabox.Client, poller *monitor.Poller, sessionTTL time.Duration) http.Handler {
	if sessionTTL <= 0 {
		sessionTTL = 7 * 24 * time.Hour
	}
	sfs := web.StaticFS()
	indexBytes, err := fs.ReadFile(sfs, "index.html")
	if err != nil {
		indexBytes = []byte("<!doctype html><html><body>Missing UI</body></html>")
	}

	s := &Server{
		store:        st,
		client:       client,
		poller:       poller,
		sessionTTL:   sessionTTL,
		staticFS:     sfs,
		indexTmpl:    string(indexBytes),
		loginLimiter: newRateLimiter(12, time.Minute),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /manifest.webmanifest", s.handleManifest)
	mux.HandleFunc("GET /sw.js", s.handleStaticFile("sw.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("GET /css/", s.handleStaticDir)
	mux.HandleFunc("GET /js/", s.handleStaticDir)
	mux.HandleFunc("GET /assets/", s.handleStaticDir)

	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/setup", s.withFetchMetadata(s.handleAuthSetup))
	mux.HandleFunc("POST /api/auth/login", s.withFetchMetadata(s.handleAuthLogin))
	mux.HandleFunc("POST /api/auth/logout", s.withAuth(true, s.handleAuthLogout))

	mux.HandleFunc("GET /api/accounts", s.withAuth(false, s.handleListAccounts))
	mux.HandleFunc("POST /api/accounts", s.withAuth(true, s.handleCreateAccount))
	mux.HandleFunc("PATCH /api/accounts/{id}", s.withAuth(true, s.handleUpdateAccount))
	mux.HandleFunc("DELETE /api/accounts/{id}", s.withAuth(true, s.handleDeleteAccount))
	mux.HandleFunc("POST /api/accounts/{id}/sync", s.withAuth(true, s.handleSyncAccount))
	mux.HandleFunc("GET /api/accounts/{id}/machine-options", s.withAuth(false, s.handleAccountMachineOptions))

	mux.HandleFunc("GET /api/machines", s.withAuth(false, s.handleListMachines))
	mux.HandleFunc("POST /api/machines", s.withAuth(true, s.handleCreateMachine))
	mux.HandleFunc("PATCH /api/machines/{accountId}/{machineId}", s.withAuth(true, s.handleRenameMachine))
	mux.HandleFunc("PATCH /api/machines/{accountId}/{machineId}/keepalive", s.withAuth(true, s.handleSetMachineKeepalive))
	mux.HandleFunc("DELETE /api/machines/{accountId}/{machineId}", s.withAuth(true, s.handleDeleteMachine))
	mux.HandleFunc("POST /api/sync", s.withAuth(true, s.handleSyncAll))
	mux.HandleFunc("GET /api/machines/{accountId}/{machineId}/telemetry", s.withAuth(false, s.handleMachineTelemetry))
	mux.HandleFunc("POST /api/machines/{accountId}/{machineId}/actions/{action}", s.withAuth(true, s.handleMachineAction))

	mux.HandleFunc("GET /", s.handleIndex)

	return s.securityHeaders(mux)
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleManifest(w http.ResponseWriter, _ *http.Request) {
	data, err := fs.ReadFile(s.staticFS, "manifest.webmanifest")
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(data)
}

func (s *Server) handleStaticFile(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(s.staticFS, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	}
}

func (s *Server) handleStaticDir(w http.ResponseWriter, r *http.Request) {
	clean := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if clean == "" || strings.HasPrefix(clean, "..") || clean == "index.html" {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(s.staticFS, clean)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(path.Ext(clean))
	ct := mime.TypeByExtension(ext)
	switch ext {
	case ".css":
		ct = "text/css; charset=utf-8"
	case ".js", ".mjs":
		ct = "application/javascript; charset=utf-8"
	case ".svg":
		ct = "image/svg+xml"
	case ".png":
		ct = "image/png"
	}
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(data)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	nonce, _, err := crypto.RandomToken(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'nonce-" + nonce + "'",
		"style-src 'self' https://fonts.googleapis.com 'unsafe-inline'",
		"font-src 'self' https://fonts.gstatic.com",
		"img-src 'self' data: https:",
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
	}, "; ")
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	html := strings.ReplaceAll(s.indexTmpl, "{{NONCE}}", nonce)
	_, _ = w.Write([]byte(html))
}

func (s *Server) withFetchMetadata(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Sec-Fetch-Dest, Sec-Fetch-Mode, Sec-Fetch-Site")
		site := r.Header.Get("Sec-Fetch-Site")
		if site != "" && site != "same-origin" && site != "none" {
			writeError(w, http.StatusForbidden, "cross_site_forbidden", "cross-site request rejected")
			return
		}
		next(w, r)
	}
}

type sessKey struct{}

func (s *Server) withAuth(requireCSRF bool, next http.HandlerFunc) http.HandlerFunc {
	return s.withFetchMetadata(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.currentSession(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if requireCSRF {
			got := r.Header.Get("X-CSRF-Token")
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRFToken)) != 1 {
				writeError(w, http.StatusForbidden, "csrf_invalid", "invalid or missing CSRF token")
				return
			}
		}
		ctx := context.WithValue(r.Context(), sessKey{}, sess)
		next(w, r.WithContext(ctx))
	})
}

func sessionFromContext(ctx context.Context) *store.Session {
	v, _ := ctx.Value(sessKey{}).(*store.Session)
	return v
}

func (s *Server) currentSession(r *http.Request) (*store.Session, bool) {
	for _, name := range []string{sessionCookieHTTPS, sessionCookiePlain} {
		c, err := r.Cookie(name)
		if err != nil || c.Value == "" {
			continue
		}
		sess, err := s.store.GetSession(r.Context(), c.Value)
		if err == nil && sess != nil {
			return sess, true
		}
	}
	return nil, false
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := isHTTPS(r)
	name := sessionCookiePlain
	if secure {
		name = sessionCookieHTTPS
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(s.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	secure := isHTTPS(r)
	for _, name := range []string{sessionCookieHTTPS, sessionCookiePlain} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure && name == sessionCookieHTTPS,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if isTrustedProxy(remoteHostIP(r.RemoteAddr)) {
		proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
		return proto == "https"
	}
	return false
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := s.store.HasAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to inspect admin state")
		return
	}
	resp := map[string]any{
		"needsSetup":    !hasAdmin,
		"authenticated": false,
		"serverTimeMs":  time.Now().UTC().UnixMilli(),
		"clockOffsetMs": s.poller.ClockOffsetMs(),
	}
	if sess, ok := s.currentSession(r); ok {
		resp["authenticated"] = true
		resp["username"] = sess.Username
		resp["csrfToken"] = sess.CSRFToken
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 2 || len(req.Username) > 64 {
		writeError(w, http.StatusBadRequest, "invalid_username", "username must be 2-64 characters")
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 128 {
		writeError(w, http.StatusBadRequest, "invalid_password", "password must be 8-128 characters")
		return
	}
	if err := s.store.CreateInitialAdmin(r.Context(), req.Username, req.Password); err != nil {
		if errors.Is(err, store.ErrAdminExists) {
			writeError(w, http.StatusConflict, "admin_already_exists", "initial admin account already exists")
			return
		}
		log.Printf("create initial admin error: %v", err)
		writeError(w, http.StatusInternalServerError, "setup_failed", "failed to create administrator account")
		return
	}
	admin, err := s.store.AuthenticateAdmin(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_failed", "created admin but failed login")
		return
	}
	token, sess, err := s.store.CreateSession(r.Context(), admin.ID, s.sessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", "failed to create session")
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, map[string]any{
		"authenticated": true,
		"username":      admin.Username,
		"csrfToken":     sess.CSRFToken,
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.loginLimiter.Allow(ip) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts, please wait")
		return
	}
	hasAdmin, err := s.store.HasAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to inspect admin state")
		return
	}
	if !hasAdmin {
		writeError(w, http.StatusPreconditionRequired, "setup_required", "initial admin setup required")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	admin, err := s.store.AuthenticateAdmin(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	token, sess, err := s.store.CreateSession(r.Context(), admin.ID, s.sessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", "failed to create session")
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      admin.Username,
		"csrfToken":     sess.CSRFToken,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{sessionCookieHTTPS, sessionCookiePlain} {
		if c, err := r.Cookie(name); err == nil && c.Value != "" {
			_ = s.store.DeleteSession(r.Context(), c.Value)
		}
	}
	s.clearSessionCookie(w, r)
	w.Header().Set("Clear-Site-Data", `"cookies", "cache"`)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to list accounts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Alias  string `json:"alias"`
		APIKey string `json:"apiKey"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Alias = strings.TrimSpace(req.Alias)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.Alias == "" || len(req.Alias) > 64 {
		writeError(w, http.StatusBadRequest, "invalid_alias", "alias is required (max 64 chars)")
		return
	}
	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_api_key", "apiKey is required")
		return
	}

	me, err := s.client.GetMe(r.Context(), req.APIKey)
	if err != nil {
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			status := apiErr.Status
			if status < 400 || status >= 500 {
				status = http.StatusBadGateway
			}
			writeError(w, status, apiErr.Code, apiErr.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "upstream_unreachable", "failed to verify API key with instabox.sh")
		return
	}

	in := store.AccountInput{
		Alias:          req.Alias,
		APIKey:         req.APIKey,
		InstaboxUserID: me.UserID,
		InstaboxLogin:  me.Login,
	}
	if me.Org != nil {
		in.OrgID = me.Org.ID
		in.OrgName = me.Org.Name
		in.OrgTier = me.Org.Tier
	}

	acc, err := s.store.CreateAccount(r.Context(), in)
	if err != nil {
		log.Printf("create account error: %v", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "failed to store account")
		return
	}

	// Perform initial sync right away so newly added VMs appear immediately.
	syncCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	_ = s.poller.SyncAccount(syncCtx, acc.ID)
	fresh, err := s.store.GetAccount(r.Context(), acc.ID)
	if err == nil && fresh != nil {
		acc = fresh
	}
	writeJSON(w, http.StatusCreated, acc)
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Alias   string `json:"alias"`
		APIKey  string `json:"apiKey"`
		Enabled *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Alias = strings.TrimSpace(req.Alias)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if len(req.Alias) > 64 {
		writeError(w, http.StatusBadRequest, "invalid_alias", "alias max 64 chars")
		return
	}
	if req.APIKey != "" {
		me, err := s.client.GetMe(r.Context(), req.APIKey)
		if err != nil {
			var apiErr *instabox.APIError
			if errors.As(err, &apiErr) {
				writeError(w, clampUpstreamStatus(apiErr.Status), apiErr.Code, apiErr.Message)
				return
			}
			writeError(w, http.StatusBadGateway, "upstream_unreachable", "failed to verify new API key")
			return
		}
		if err := s.store.UpdateAccount(r.Context(), id, req.Alias, req.APIKey, req.Enabled); err != nil {
			if errors.Is(err, store.ErrAccountNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "account not found")
				return
			}
			log.Printf("update account %s error: %v", id, err)
			writeError(w, http.StatusInternalServerError, "update_failed", "failed to update account")
			return
		}
		meta := store.AccountInput{InstaboxUserID: me.UserID, InstaboxLogin: me.Login}
		if me.Org != nil {
			meta.OrgID = me.Org.ID
			meta.OrgName = me.Org.Name
			meta.OrgTier = me.Org.Tier
		}
		_ = s.store.UpdateAccountSyncMeta(r.Context(), id, meta, "")
	} else {
		if err := s.store.UpdateAccount(r.Context(), id, req.Alias, "", req.Enabled); err != nil {
			if errors.Is(err, store.ErrAccountNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "account not found")
				return
			}
			log.Printf("update account %s error: %v", id, err)
			writeError(w, http.StatusInternalServerError, "update_failed", "failed to update account")
			return
		}
	}

	acc, err := s.store.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	writeJSON(w, http.StatusOK, acc)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteAccount(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "account not found")
			return
		}
		log.Printf("delete account %s error: %v", id, err)
		writeError(w, http.StatusInternalServerError, "delete_failed", "failed to delete account")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSyncAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetAccount(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	if err := s.poller.SyncAccount(r.Context(), id); err != nil {
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			writeError(w, clampUpstreamStatus(apiErr.Status), apiErr.Code, apiErr.Message)
			return
		}
		log.Printf("sync account %s error: %v", id, err)
		writeError(w, http.StatusBadGateway, "sync_failed", "failed to sync account with instabox.sh")
		return
	}
	s.handleListMachines(w, r)
}

func (s *Server) handleSyncAll(w http.ResponseWriter, r *http.Request) {
	s.poller.SyncAll(r.Context())
	s.handleListMachines(w, r)
}

func isValidMachineName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if i == 0 {
			if !isAlphaNum {
				return false
			}
			continue
		}
		if !isAlphaNum && c != '-' {
			return false
		}
	}
	return true
}

func (s *Server) handleAccountMachineOptions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	opts, err := s.poller.GetAccountMachineOptions(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "account not found")
			return
		}
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			writeError(w, clampUpstreamStatus(apiErr.Status), apiErr.Code, apiErr.Message)
			return
		}
		log.Printf("machine options %s error: %v", id, err)
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to load machine options from instabox.sh")
		return
	}
	writeJSON(w, http.StatusOK, opts)
}

func (s *Server) handleCreateMachine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID  string `json:"accountId"`
		Name       string `json:"name"`
		TemplateID string `json:"templateId"`
		VCPUs      int    `json:"vcpus"`
		MemMiB     int    `json:"memMiB"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.AccountID = strings.TrimSpace(req.AccountID)
	req.Name = strings.TrimSpace(req.Name)
	req.TemplateID = strings.TrimSpace(req.TemplateID)
	if req.AccountID == "" {
		writeError(w, http.StatusBadRequest, "invalid_account", "accountId is required")
		return
	}
	if req.Name != "" && !isValidMachineName(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid_name", "machine name must be 1-63 lowercase letters, digits or hyphens, starting with a letter or digit")
		return
	}
	if len(req.TemplateID) > 64 {
		writeError(w, http.StatusBadRequest, "invalid_template", "templateId is too long")
		return
	}
	if req.VCPUs < 0 || req.MemMiB < 0 {
		writeError(w, http.StatusBadRequest, "invalid_spec", "vcpus and memMiB must be non-negative")
		return
	}

	in := instabox.CreateMachineInput{
		Name:       req.Name,
		TemplateID: req.TemplateID,
		VCPUs:      req.VCPUs,
		MemMiB:     req.MemMiB,
	}
	res, err := s.poller.CreateMachine(r.Context(), req.AccountID, in)
	if err != nil {
		if errors.Is(err, store.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "account not found")
			return
		}
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			writeJSON(w, clampUpstreamStatus(apiErr.Status), map[string]any{
				"error": map[string]any{
					"code":              apiErr.Code,
					"message":           apiErr.Message,
					"retryable":         apiErr.Retryable,
					"operationId":       apiErr.OperationID,
					"retryAfterSeconds": apiErr.RetryAfterSeconds,
					"reason":            apiErr.Reason,
					"hint":              apiErr.Hint,
				},
			})
			return
		}
		log.Printf("create machine %s error: %v", req.AccountID, err)
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to create machine with instabox.sh")
		return
	}

	accounts, accErr := s.store.ListAccounts(r.Context())
	machines, machErr := s.store.ListMachines(r.Context())
	resp := map[string]any{
		"ok":            true,
		"operation":     res.Operation,
		"serverTimeMs":  time.Now().UTC().UnixMilli(),
		"clockOffsetMs": s.poller.ClockOffsetMs(),
	}
	if accErr == nil && accounts != nil {
		resp["accounts"] = accounts
	}
	if machErr == nil && machines != nil {
		resp["machines"] = machines
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) handleRenameMachine(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accountId")
	machineID := r.PathValue("machineId")
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !isValidMachineName(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid_name", "machine name must be 1-63 lowercase letters, digits or hyphens, starting with a letter or digit")
		return
	}
	updated, err := s.poller.RenameMachine(r.Context(), accountID, machineID, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrAccountNotFound) || errors.Is(err, store.ErrMachineNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			writeJSON(w, clampUpstreamStatus(apiErr.Status), map[string]any{
				"error": map[string]any{
					"code":              apiErr.Code,
					"message":           apiErr.Message,
					"retryable":         apiErr.Retryable,
					"operationId":       apiErr.OperationID,
					"retryAfterSeconds": apiErr.RetryAfterSeconds,
					"reason":            apiErr.Reason,
					"hint":              apiErr.Hint,
				},
			})
			return
		}
		log.Printf("rename machine %s/%s error: %v", accountID, machineID, err)
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to rename machine on instabox.sh")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"machine": updated,
	})
}

func (s *Server) handleSetMachineKeepalive(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accountId")
	machineID := r.PathValue("machineId")
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "enabled boolean is required")
		return
	}
	updated, err := s.poller.SetMachineKeepalive(r.Context(), accountID, machineID, *req.Enabled)
	if err != nil {
		if errors.Is(err, store.ErrAccountNotFound) || errors.Is(err, store.ErrMachineNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		log.Printf("set machine keepalive %s/%s error: %v", accountID, machineID, err)
		writeError(w, http.StatusInternalServerError, "keepalive_failed", "failed to update keepalive setting")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"machine": updated,
	})
}

func (s *Server) handleDeleteMachine(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accountId")
	machineID := r.PathValue("machineId")
	res, err := s.poller.DeleteMachine(r.Context(), accountID, machineID)
	if err != nil {
		if errors.Is(err, store.ErrAccountNotFound) || errors.Is(err, store.ErrMachineNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			writeJSON(w, clampUpstreamStatus(apiErr.Status), map[string]any{
				"error": map[string]any{
					"code":              apiErr.Code,
					"message":           apiErr.Message,
					"retryable":         apiErr.Retryable,
					"operationId":       apiErr.OperationID,
					"retryAfterSeconds": apiErr.RetryAfterSeconds,
					"reason":            apiErr.Reason,
					"hint":              apiErr.Hint,
				},
			})
			return
		}
		log.Printf("delete machine %s/%s error: %v", accountID, machineID, err)
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to delete machine on instabox.sh")
		return
	}
	accounts, accErr := s.store.ListAccounts(r.Context())
	machines, machErr := s.store.ListMachines(r.Context())
	resp := map[string]any{
		"ok":            true,
		"operation":     res.Operation,
		"serverTimeMs":  time.Now().UTC().UnixMilli(),
		"clockOffsetMs": s.poller.ClockOffsetMs(),
	}
	if accErr == nil && accounts != nil {
		resp["accounts"] = accounts
	}
	if machErr == nil && machines != nil {
		resp["machines"] = machines
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListMachines(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to list accounts")
		return
	}
	machines, err := s.store.ListMachines(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to list machines")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"serverTimeMs":  time.Now().UTC().UnixMilli(),
		"clockOffsetMs": s.poller.ClockOffsetMs(),
		"accounts":      accounts,
		"machines":      machines,
	})
}

func (s *Server) handleMachineTelemetry(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accountId")
	machineID := r.PathValue("machineId")
	m, err := s.store.GetMachine(r.Context(), accountID, machineID)
	if err != nil {
		if errors.Is(err, store.ErrMachineNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "machine not found")
			return
		}
		log.Printf("get machine telemetry %s/%s error: %v", accountID, machineID, err)
		writeError(w, http.StatusInternalServerError, "db_error", "failed to load machine")
		return
	}

	window := 1 * time.Hour
	switch r.URL.Query().Get("range") {
	case "15m":
		window = 15 * time.Minute
	case "1h", "":
		window = 1 * time.Hour
	case "6h":
		window = 6 * time.Hour
	case "24h":
		window = 24 * time.Hour
	case "7d":
		window = 7 * 24 * time.Hour
	}

	since := time.Now().Add(-window).Unix()
	samples, err := s.store.GetMachineMetrics(r.Context(), accountID, machineID, since)
	if err != nil {
		log.Printf("get machine metrics %s/%s error: %v", accountID, machineID, err)
		writeError(w, http.StatusInternalServerError, "db_error", "failed to load telemetry")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"machine":      m,
		"samples":      samples,
		"serverTimeMs": time.Now().UTC().UnixMilli(),
	})
}

func (s *Server) handleMachineAction(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accountId")
	machineID := r.PathValue("machineId")
	actionRaw := r.PathValue("action")

	var action instabox.Action
	switch instabox.Action(actionRaw) {
	case instabox.ActionWake, instabox.ActionSuspend, instabox.ActionStart, instabox.ActionStop:
		action = instabox.Action(actionRaw)
	default:
		writeError(w, http.StatusBadRequest, "invalid_action", "allowed actions: wake, suspend, start, stop")
		return
	}

	res, err := s.poller.ExecuteAction(r.Context(), accountID, machineID, action)
	if err != nil {
		if errors.Is(err, store.ErrAccountNotFound) || errors.Is(err, store.ErrMachineNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		var apiErr *instabox.APIError
		if errors.As(err, &apiErr) {
			writeJSON(w, clampUpstreamStatus(apiErr.Status), map[string]any{
				"error": map[string]any{
					"code":              apiErr.Code,
					"message":           apiErr.Message,
					"retryable":         apiErr.Retryable,
					"operationId":       apiErr.OperationID,
					"retryAfterSeconds": apiErr.RetryAfterSeconds,
					"reason":            apiErr.Reason,
					"hint":              apiErr.Hint,
				},
			})
			return
		}
		log.Printf("machine action %s %s/%s error: %v", actionRaw, accountID, machineID, err)
		writeError(w, http.StatusBadGateway, "upstream_error", "upstream action request failed")
		return
	}

	fresh, _ := s.store.GetMachine(r.Context(), accountID, machineID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"action":    actionRaw,
		"result":    res,
		"machine":   fresh,
		"sessionId": sessionFromContext(r.Context()).Username,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid JSON payload")
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "bad_json", "unexpected trailing JSON data")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func clampUpstreamStatus(status int) int {
	if status < 400 || status >= 600 {
		return http.StatusBadGateway
	}
	return status
}

var trustedProxyNets = func() []*net.IPNet {
	cidrs := []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}
	var out []*net.IPNet
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}()

func isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range trustedProxyNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteHostIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		if parsed := net.ParseIP(host); parsed != nil {
			return parsed.String()
		}
		return host
	}
	trimmed := strings.TrimSpace(remoteAddr)
	if parsed := net.ParseIP(trimmed); parsed != nil {
		return parsed.String()
	}
	return trimmed
}

// clientIP resolves the peer IP, consulting X-Forwarded-For right-to-left ONLY
// when the direct socket peer is a trusted private/loopback reverse proxy.
func clientIP(r *http.Request) string {
	directIP := remoteHostIP(r.RemoteAddr)
	if !isTrustedProxy(directIP) {
		return directIP
	}
	if vals := r.Header.Values("X-Forwarded-For"); len(vals) > 0 {
		xff := strings.Join(vals, ",")
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			cand := strings.TrimSpace(parts[i])
			if parsed := net.ParseIP(cand); parsed != nil && !isTrustedProxy(cand) {
				return parsed.String()
			}
		}
		for i := len(parts) - 1; i >= 0; i-- {
			cand := strings.TrimSpace(parts[i])
			if parsed := net.ParseIP(cand); parsed != nil {
				return parsed.String()
			}
		}
	}
	return directIP
}

type rateLimiter struct {
	mu          sync.Mutex
	maxHits     int
	window      time.Duration
	entries     map[string][]time.Time
	lastCleanup time.Time
}

func newRateLimiter(maxHits int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		maxHits:     maxHits,
		window:      window,
		entries:     make(map[string][]time.Time),
		lastCleanup: time.Now(),
	}
}

func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	if now.Sub(rl.lastCleanup) >= rl.window || len(rl.entries) > 2048 {
		for k, times := range rl.entries {
			kept := times[:0]
			for _, t := range times {
				if t.After(cutoff) {
					kept = append(kept, t)
				}
			}
			if len(kept) == 0 {
				delete(rl.entries, k)
			} else {
				rl.entries[k] = kept
			}
		}
		rl.lastCleanup = now
		// Evict oldest active entries down to 2048 instead of wiping active attackers' counters.
		if len(rl.entries) > 4096 {
			type keyTime struct {
				key string
				ts  time.Time
			}
			items := make([]keyTime, 0, len(rl.entries))
			for k, times := range rl.entries {
				var latest time.Time
				if len(times) > 0 {
					latest = times[len(times)-1]
				}
				items = append(items, keyTime{key: k, ts: latest})
			}
			sort.Slice(items, func(i, j int) bool { return items[i].ts.Before(items[j].ts) })
			for _, it := range items[:len(items)-2048] {
				delete(rl.entries, it.key)
			}
		}
	}
	prev := rl.entries[key]
	kept := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.maxHits {
		rl.entries[key] = kept
		return false
	}
	rl.entries[key] = append(kept, now)
	return true
}
