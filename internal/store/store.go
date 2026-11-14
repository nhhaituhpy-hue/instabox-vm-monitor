package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	_ "modernc.org/sqlite"
)

var (
	ErrAdminExists        = errors.New("admin account already configured")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrSessionNotFound    = errors.New("session not found or expired")
	ErrAccountNotFound    = errors.New("account not found")
	ErrMachineNotFound    = errors.New("machine not found")
)

type Store struct {
	db        *sql.DB
	masterKey []byte
}

type AdminUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"createdAt"`
}

type Session struct {
	TokenHash string `json:"-"`
	AdminID   int64  `json:"adminId"`
	Username  string `json:"username"`
	CSRFToken string `json:"csrfToken"`
	ExpiresAt int64  `json:"expiresAt"`
	CreatedAt int64  `json:"createdAt"`
}

type Account struct {
	ID             string          `json:"id"`
	Alias          string          `json:"alias"`
	KeyMask        string          `json:"keyMask"`
	InstaboxUserID int64           `json:"instaboxUserId"`
	InstaboxLogin  string          `json:"instaboxLogin"`
	OrgID          string          `json:"orgId"`
	OrgName        string          `json:"orgName"`
	OrgTier        string          `json:"orgTier"`
	OrgUsage       json.RawMessage `json:"orgUsage,omitempty"`
	Enabled        bool            `json:"enabled"`
	LastSyncAt     int64           `json:"lastSyncAt"`
	LastError      string          `json:"lastError"`
	CreatedAt      int64           `json:"createdAt"`
	UpdatedAt      int64           `json:"updatedAt"`
}

type AccountInput struct {
	Alias          string
	APIKey         string
	InstaboxUserID int64
	InstaboxLogin  string
	OrgID          string
	OrgName        string
	OrgTier        string
	OrgUsageJSON   string
}

type MachineRecord struct {
	AccountID               string          `json:"accountId"`
	AccountAlias            string          `json:"accountAlias,omitempty"`
	AccountOrgTier          string          `json:"accountOrgTier,omitempty"`
	MachineID               string          `json:"id"`
	Name                    string          `json:"name"`
	State                   string          `json:"state"`
	DesiredState            string          `json:"desiredState"`
	VCPUs                   int             `json:"vcpus"`
	MemMiB                  int             `json:"memMiB"`
	PersistentDiskGiB       int             `json:"persistentDiskGiB"`
	RootDiskGiB             int             `json:"rootDiskGiB"`
	TemplateID              string          `json:"templateId"`
	Free                    bool            `json:"free"`
	Ready                   bool            `json:"ready"`
	ProcessesPreserved      bool            `json:"processesPreserved"`
	AutoSuspendAfterSeconds *int            `json:"autoSuspendAfterSeconds"`
	MaxRunMinutes           int             `json:"maxRunMinutes"`
	RunStartedAt            string          `json:"runStartedAt"`
	AutoSuspendedAt         string          `json:"autoSuspendedAt"`
	CooldownUntil           string          `json:"cooldownUntil"`
	CooldownReason          string          `json:"cooldownReason"`
	OperationID             string          `json:"operationId"`
	MonthlyRunningSeconds   int64           `json:"monthlyRunningSeconds"`
	LastErrorCode           string          `json:"lastErrorCode"`
	LastErrorMsg            string          `json:"lastErrorMsg"`
	DiskJSON                json.RawMessage `json:"disk,omitempty"`
	RawJSON                 json.RawMessage `json:"-"`
	LatestMetric            *MetricSample   `json:"latestMetric,omitempty"`
	RecentMetrics           []MetricSample  `json:"recentMetrics,omitempty"`
	UpdatedAt               int64           `json:"updatedAt"`
}

type MetricSample struct {
	AccountID    string  `json:"accountId,omitempty"`
	MachineID    string  `json:"machineId,omitempty"`
	TS           int64   `json:"ts"`
	CPUPct       float64 `json:"cpuPct"`
	CPUCount     int     `json:"cpuCount"`
	MemUsed      int64   `json:"memUsed"`
	MemTotal     int64   `json:"memTotal"`
	DiskUsed     int64   `json:"diskUsed"`
	DiskTotal    int64   `json:"diskTotal"`
	RootUsed     int64   `json:"rootUsed"`
	RootTotal    int64   `json:"rootTotal"`
	NetInBPS     float64 `json:"netInBps"`
	NetOutBPS    float64 `json:"netOutBps"`
	DiskReadBPS  float64 `json:"diskReadBps"`
	DiskWriteBPS float64 `json:"diskWriteBps"`
}

func Open(dbPath string, masterKey []byte) (*Store, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &Store{db: db, masterKey: append([]byte(nil), masterKey...)}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
	CREATE TABLE IF NOT EXISTS admin_users (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token_hash TEXT PRIMARY KEY,
		admin_id INTEGER NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
		csrf_token TEXT NOT NULL,
		expires_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		alias TEXT NOT NULL,
		key_enc TEXT NOT NULL,
		key_mask TEXT NOT NULL,
		instabox_user_id INTEGER NOT NULL DEFAULT 0,
		instabox_login TEXT NOT NULL DEFAULT '',
		org_id TEXT NOT NULL DEFAULT '',
		org_name TEXT NOT NULL DEFAULT '',
		org_tier TEXT NOT NULL DEFAULT '',
		org_usage_json TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		last_sync_at INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS machines (
		account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		machine_id TEXT NOT NULL,
		name TEXT NOT NULL,
		state TEXT NOT NULL,
		desired_state TEXT NOT NULL,
		vcpus INTEGER NOT NULL DEFAULT 0,
		mem_mib INTEGER NOT NULL DEFAULT 0,
		persistent_disk_gib INTEGER NOT NULL DEFAULT 0,
		root_disk_gib INTEGER NOT NULL DEFAULT 0,
		template_id TEXT NOT NULL DEFAULT '',
		free INTEGER NOT NULL DEFAULT 0,
		ready INTEGER NOT NULL DEFAULT 0,
		processes_preserved INTEGER NOT NULL DEFAULT 0,
		auto_suspend_after_seconds INTEGER,
		max_run_minutes INTEGER NOT NULL DEFAULT 0,
		run_started_at TEXT NOT NULL DEFAULT '',
		auto_suspended_at TEXT NOT NULL DEFAULT '',
		cooldown_until TEXT NOT NULL DEFAULT '',
		cooldown_reason TEXT NOT NULL DEFAULT '',
		operation_id TEXT NOT NULL DEFAULT '',
		monthly_running_seconds INTEGER NOT NULL DEFAULT 0,
		last_error_code TEXT NOT NULL DEFAULT '',
		last_error_msg TEXT NOT NULL DEFAULT '',
		disk_json TEXT NOT NULL DEFAULT '',
		raw_json TEXT NOT NULL DEFAULT '',
		updated_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (account_id, machine_id)
	);

	CREATE TABLE IF NOT EXISTS machine_metrics (
		account_id TEXT NOT NULL,
		machine_id TEXT NOT NULL,
		ts INTEGER NOT NULL,
		cpu_pct REAL NOT NULL DEFAULT 0,
		cpu_count INTEGER NOT NULL DEFAULT 0,
		mem_used INTEGER NOT NULL DEFAULT 0,
		mem_total INTEGER NOT NULL DEFAULT 0,
		disk_used INTEGER NOT NULL DEFAULT 0,
		disk_total INTEGER NOT NULL DEFAULT 0,
		root_used INTEGER NOT NULL DEFAULT 0,
		root_total INTEGER NOT NULL DEFAULT 0,
		net_in_bps REAL NOT NULL DEFAULT 0,
		net_out_bps REAL NOT NULL DEFAULT 0,
		disk_read_bps REAL NOT NULL DEFAULT 0,
		disk_write_bps REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (account_id, machine_id, ts),
		FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_machine_metrics_ts ON machine_metrics(account_id, machine_id, ts);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count)
	return count > 0, err
}

func (s *Store) CreateInitialAdmin(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if len(username) < 2 || len(username) > 64 {
		return errors.New("invalid username length")
	}
	if len(password) < 8 || len(password) > 128 {
		return errors.New("invalid password length")
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO admin_users(id, username, password_hash, created_at) VALUES (1, ?, ?, ?)`,
		username, hash, now,
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrAdminExists
	}
	return nil
}

var dummyAdminHash, _ = crypto.HashPassword("dummy-constant-time-verification-placeholder")

func (s *Store) AuthenticateAdmin(ctx context.Context, username, password string) (*AdminUser, error) {
	username = strings.TrimSpace(username)
	var u AdminUser
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at FROM admin_users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &hash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		_ = crypto.VerifyPassword(password, dummyAdminHash)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !crypto.VerifyPassword(password, hash) {
		return nil, ErrInvalidCredentials
	}
	return &u, nil
}

func (s *Store) CreateSession(ctx context.Context, adminID int64, ttl time.Duration) (string, *Session, error) {
	token, tokenHash, err := crypto.RandomToken(32)
	if err != nil {
		return "", nil, err
	}
	csrf, _, err := crypto.RandomToken(24)
	if err != nil {
		return "", nil, err
	}
	now := time.Now().Unix()
	exp := now + int64(ttl.Seconds())
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions(token_hash, admin_id, csrf_token, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		tokenHash, adminID, csrf, exp, now,
	)
	if err != nil {
		return "", nil, err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now)
	return token, &Session{
		TokenHash: tokenHash,
		AdminID:   adminID,
		CSRFToken: csrf,
		ExpiresAt: exp,
		CreatedAt: now,
	}, nil
}

func (s *Store) GetSession(ctx context.Context, rawToken string) (*Session, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, ErrSessionNotFound
	}
	hash := crypto.HashToken(rawToken)
	now := time.Now().Unix()
	var sess Session
	err := s.db.QueryRowContext(ctx, `
		SELECT s.token_hash, s.admin_id, a.username, s.csrf_token, s.expires_at, s.created_at
		FROM sessions s
		JOIN admin_users a ON a.id = s.admin_id
		WHERE s.token_hash = ? AND s.expires_at > ?
	`, hash, now).Scan(&sess.TokenHash, &sess.AdminID, &sess.Username, &sess.CSRFToken, &sess.ExpiresAt, &sess.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, crypto.HashToken(rawToken))
	return err
}

func (s *Store) CreateAccount(ctx context.Context, in AccountInput) (*Account, error) {
	alias := strings.TrimSpace(in.Alias)
	apiKey := strings.TrimSpace(in.APIKey)
	if alias == "" || apiKey == "" {
		return nil, errors.New("alias and apiKey are required")
	}
	enc, err := crypto.EncryptSecret(s.masterKey, apiKey)
	if err != nil {
		return nil, err
	}
	mask := crypto.MaskAPIKey(apiKey)
	id := crypto.NewUUID()
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO accounts(
			id, alias, key_enc, key_mask, instabox_user_id, instabox_login,
			org_id, org_name, org_tier, org_usage_json, enabled, last_sync_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 0, '', ?, ?)
	`, id, alias, enc, mask, in.InstaboxUserID, in.InstaboxLogin, in.OrgID, in.OrgName, in.OrgTier, in.OrgUsageJSON, now, now)
	if err != nil {
		return nil, err
	}
	return s.GetAccount(ctx, id)
}

func (s *Store) GetAccount(ctx context.Context, id string) (*Account, error) {
	var a Account
	var enabledInt int
	var usageStr string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, alias, key_mask, instabox_user_id, instabox_login, org_id, org_name, org_tier,
		       org_usage_json, enabled, last_sync_at, last_error, created_at, updated_at
		FROM accounts WHERE id = ?
	`, id).Scan(
		&a.ID, &a.Alias, &a.KeyMask, &a.InstaboxUserID, &a.InstaboxLogin,
		&a.OrgID, &a.OrgName, &a.OrgTier, &usageStr, &enabledInt,
		&a.LastSyncAt, &a.LastError, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Enabled = enabledInt != 0
	if usageStr != "" {
		a.OrgUsage = json.RawMessage(usageStr)
	}
	return &a, nil
}

func (s *Store) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, alias, key_mask, instabox_user_id, instabox_login, org_id, org_name, org_tier,
		       org_usage_json, enabled, last_sync_at, last_error, created_at, updated_at
		FROM accounts ORDER BY created_at ASC, alias ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Account, 0)
	for rows.Next() {
		var a Account
		var enabledInt int
		var usageStr string
		if err := rows.Scan(
			&a.ID, &a.Alias, &a.KeyMask, &a.InstaboxUserID, &a.InstaboxLogin,
			&a.OrgID, &a.OrgName, &a.OrgTier, &usageStr, &enabledInt,
			&a.LastSyncAt, &a.LastError, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		a.Enabled = enabledInt != 0
		if usageStr != "" {
			a.OrgUsage = json.RawMessage(usageStr)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAccountAPIKey(ctx context.Context, id string) (string, error) {
	var enc string
	err := s.db.QueryRowContext(ctx, `SELECT key_enc FROM accounts WHERE id = ?`, id).Scan(&enc)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAccountNotFound
	}
	if err != nil {
		return "", err
	}
	return crypto.DecryptSecret(s.masterKey, enc)
}

func (s *Store) UpdateAccount(ctx context.Context, id, alias, newAPIKey string, enabled *bool) error {
	acc, err := s.GetAccount(ctx, id)
	if err != nil {
		return err
	}
	if trimmed := strings.TrimSpace(alias); trimmed != "" {
		acc.Alias = trimmed
	}
	if enabled != nil {
		acc.Enabled = *enabled
	}
	now := time.Now().Unix()
	enabledInt := 0
	if acc.Enabled {
		enabledInt = 1
	}
	if trimmedKey := strings.TrimSpace(newAPIKey); trimmedKey != "" {
		enc, err := crypto.EncryptSecret(s.masterKey, trimmedKey)
		if err != nil {
			return err
		}
		mask := crypto.MaskAPIKey(trimmedKey)
		_, err = s.db.ExecContext(ctx, `
			UPDATE accounts SET alias = ?, key_enc = ?, key_mask = ?, enabled = ?, updated_at = ? WHERE id = ?
		`, acc.Alias, enc, mask, enabledInt, now, id)
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE accounts SET alias = ?, enabled = ?, updated_at = ? WHERE id = ?
	`, acc.Alias, enabledInt, now, id)
	return err
}

func (s *Store) UpdateAccountSyncMeta(ctx context.Context, id string, meta AccountInput, lastErr string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE accounts
		SET instabox_user_id = CASE WHEN ? > 0 THEN ? ELSE instabox_user_id END,
		    instabox_login = CASE WHEN ? != '' THEN ? ELSE instabox_login END,
		    org_id = CASE WHEN ? != '' THEN ? ELSE org_id END,
		    org_name = CASE WHEN ? != '' THEN ? ELSE org_name END,
		    org_tier = CASE WHEN ? != '' THEN ? ELSE org_tier END,
		    org_usage_json = CASE WHEN ? != '' THEN ? ELSE org_usage_json END,
		    last_sync_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?
	`, meta.InstaboxUserID, meta.InstaboxUserID,
		meta.InstaboxLogin, meta.InstaboxLogin,
		meta.OrgID, meta.OrgID,
		meta.OrgName, meta.OrgName,
		meta.OrgTier, meta.OrgTier,
		meta.OrgUsageJSON, meta.OrgUsageJSON,
		now, lastErr, now, id)
	return err
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func (s *Store) SyncAccountMachines(ctx context.Context, accountID string, records []MachineRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	seen := make([]string, 0, len(records))
	for _, r := range records {
		seen = append(seen, r.MachineID)
		freeInt, readyInt, procInt := boolInt(r.Free), boolInt(r.Ready), boolInt(r.ProcessesPreserved)
		var autoSusp sql.NullInt64
		if r.AutoSuspendAfterSeconds != nil {
			autoSusp = sql.NullInt64{Int64: int64(*r.AutoSuspendAfterSeconds), Valid: true}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO machines(
				account_id, machine_id, name, state, desired_state, vcpus, mem_mib, persistent_disk_gib, root_disk_gib,
				template_id, free, ready, processes_preserved, auto_suspend_after_seconds, max_run_minutes,
				run_started_at, auto_suspended_at, cooldown_until, cooldown_reason, operation_id,
				monthly_running_seconds, last_error_code, last_error_msg, disk_json, raw_json, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(account_id, machine_id) DO UPDATE SET
				name = excluded.name,
				state = excluded.state,
				desired_state = excluded.desired_state,
				vcpus = excluded.vcpus,
				mem_mib = excluded.mem_mib,
				persistent_disk_gib = excluded.persistent_disk_gib,
				root_disk_gib = excluded.root_disk_gib,
				template_id = excluded.template_id,
				free = excluded.free,
				ready = excluded.ready,
				processes_preserved = excluded.processes_preserved,
				auto_suspend_after_seconds = excluded.auto_suspend_after_seconds,
				max_run_minutes = excluded.max_run_minutes,
				run_started_at = excluded.run_started_at,
				auto_suspended_at = excluded.auto_suspended_at,
				cooldown_until = excluded.cooldown_until,
				cooldown_reason = excluded.cooldown_reason,
				operation_id = excluded.operation_id,
				monthly_running_seconds = CASE
					WHEN excluded.monthly_running_seconds > 0 THEN excluded.monthly_running_seconds
					ELSE machines.monthly_running_seconds
				END,
				last_error_code = excluded.last_error_code,
				last_error_msg = excluded.last_error_msg,
				disk_json = CASE
					WHEN excluded.disk_json != '' THEN excluded.disk_json
					ELSE machines.disk_json
				END,
				raw_json = CASE
					WHEN excluded.raw_json != '' THEN excluded.raw_json
					ELSE machines.raw_json
				END,
				updated_at = excluded.updated_at
		`, accountID, r.MachineID, r.Name, r.State, r.DesiredState, r.VCPUs, r.MemMiB, r.PersistentDiskGiB, r.RootDiskGiB,
			r.TemplateID, freeInt, readyInt, procInt, autoSusp, r.MaxRunMinutes,
			r.RunStartedAt, r.AutoSuspendedAt, r.CooldownUntil, r.CooldownReason, r.OperationID,
			r.MonthlyRunningSeconds, r.LastErrorCode, r.LastErrorMsg, string(r.DiskJSON), string(r.RawJSON), now)
		if err != nil {
			return err
		}
	}

	if len(seen) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM machine_metrics WHERE account_id = ?`, accountID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM machines WHERE account_id = ?`, accountID); err != nil {
			return err
		}
	} else {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(seen)), ",")
		args := make([]any, 0, len(seen)+1)
		args = append(args, accountID)
		for _, id := range seen {
			args = append(args, id)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM machine_metrics WHERE account_id = ? AND machine_id NOT IN (`+placeholders+`)`, args...); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM machines WHERE account_id = ? AND machine_id NOT IN (`+placeholders+`)`, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteMachine(ctx context.Context, accountID, machineID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM machines WHERE account_id = ? AND machine_id = ?`, accountID, machineID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrMachineNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM machine_metrics WHERE account_id = ? AND machine_id = ?`, accountID, machineID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateMachineDiskAndUsage(ctx context.Context, accountID, machineID, diskJSON string, monthlyRunning int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE machines
		SET disk_json = CASE WHEN ? != '' THEN ? ELSE disk_json END,
		    monthly_running_seconds = CASE WHEN ? >= 0 THEN ? ELSE monthly_running_seconds END,
		    updated_at = ?
		WHERE account_id = ? AND machine_id = ?
	`, diskJSON, diskJSON, monthlyRunning, monthlyRunning, time.Now().Unix(), accountID, machineID)
	return err
}

func (s *Store) GetMachine(ctx context.Context, accountID, machineID string) (*MachineRecord, error) {
	list, err := s.queryMachines(ctx, `WHERE m.account_id = ? AND m.machine_id = ?`, accountID, machineID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrMachineNotFound
	}
	return &list[0], nil
}

func (s *Store) ListMachines(ctx context.Context) ([]MachineRecord, error) {
	return s.queryMachines(ctx, `ORDER BY a.created_at ASC, m.name ASC, m.machine_id ASC`)
}

func (s *Store) queryMachines(ctx context.Context, clause string, args ...any) ([]MachineRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.account_id, a.alias, a.org_tier, m.machine_id, m.name, m.state, m.desired_state,
		       m.vcpus, m.mem_mib, m.persistent_disk_gib, m.root_disk_gib, m.template_id,
		       m.free, m.ready, m.processes_preserved, m.auto_suspend_after_seconds, m.max_run_minutes,
		       m.run_started_at, m.auto_suspended_at, m.cooldown_until, m.cooldown_reason, m.operation_id,
		       m.monthly_running_seconds, m.last_error_code, m.last_error_msg, m.disk_json, m.updated_at
		FROM machines m
		JOIN accounts a ON a.id = m.account_id
	`+" "+clause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MachineRecord, 0)
	for rows.Next() {
		var r MachineRecord
		var freeInt, readyInt, procInt int
		var autoSusp sql.NullInt64
		var diskStr string
		if err := rows.Scan(
			&r.AccountID, &r.AccountAlias, &r.AccountOrgTier, &r.MachineID, &r.Name, &r.State, &r.DesiredState,
			&r.VCPUs, &r.MemMiB, &r.PersistentDiskGiB, &r.RootDiskGiB, &r.TemplateID,
			&freeInt, &readyInt, &procInt, &autoSusp, &r.MaxRunMinutes,
			&r.RunStartedAt, &r.AutoSuspendedAt, &r.CooldownUntil, &r.CooldownReason, &r.OperationID,
			&r.MonthlyRunningSeconds, &r.LastErrorCode, &r.LastErrorMsg, &diskStr, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.Free = freeInt != 0
		r.Ready = readyInt != 0
		r.ProcessesPreserved = procInt != 0
		if autoSusp.Valid {
			v := int(autoSusp.Int64)
			r.AutoSuspendAfterSeconds = &v
		}
		if diskStr != "" {
			r.DiskJSON = json.RawMessage(diskStr)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		recent, err := s.getRecentMetrics(ctx, out[i].AccountID, out[i].MachineID, 36)
		if err == nil && len(recent) > 0 {
			out[i].RecentMetrics = recent
			latest := recent[len(recent)-1]
			out[i].LatestMetric = &latest
		}
	}
	return out, nil
}

func (s *Store) getRecentMetrics(ctx context.Context, accountID, machineID string, limit int) ([]MetricSample, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, cpu_pct, cpu_count, mem_used, mem_total,
		       disk_used, disk_total, root_used, root_total, net_in_bps, net_out_bps, disk_read_bps, disk_write_bps
		FROM (
			SELECT ts, cpu_pct, cpu_count, mem_used, mem_total,
			       disk_used, disk_total, root_used, root_total, net_in_bps, net_out_bps, disk_read_bps, disk_write_bps
			FROM machine_metrics
			WHERE account_id = ? AND machine_id = ?
			ORDER BY ts DESC LIMIT ?
		) sub
		ORDER BY ts ASC
	`, accountID, machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MetricSample
	for rows.Next() {
		var m MetricSample
		m.AccountID = accountID
		m.MachineID = machineID
		if err := rows.Scan(
			&m.TS, &m.CPUPct, &m.CPUCount, &m.MemUsed, &m.MemTotal,
			&m.DiskUsed, &m.DiskTotal, &m.RootUsed, &m.RootTotal,
			&m.NetInBPS, &m.NetOutBPS, &m.DiskReadBPS, &m.DiskWriteBPS,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UpsertMetrics(ctx context.Context, samples []MetricSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO machine_metrics(
			account_id, machine_id, ts, cpu_pct, cpu_count, mem_used, mem_total,
			disk_used, disk_total, root_used, root_total, net_in_bps, net_out_bps, disk_read_bps, disk_write_bps
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, machine_id, ts) DO UPDATE SET
			cpu_pct = excluded.cpu_pct,
			cpu_count = excluded.cpu_count,
			mem_used = excluded.mem_used,
			mem_total = excluded.mem_total,
			disk_used = CASE WHEN excluded.disk_used > 0 THEN excluded.disk_used ELSE machine_metrics.disk_used END,
			disk_total = CASE WHEN excluded.disk_total > 0 THEN excluded.disk_total ELSE machine_metrics.disk_total END,
			root_used = CASE WHEN excluded.root_used > 0 THEN excluded.root_used ELSE machine_metrics.root_used END,
			root_total = CASE WHEN excluded.root_total > 0 THEN excluded.root_total ELSE machine_metrics.root_total END,
			net_in_bps = excluded.net_in_bps,
			net_out_bps = excluded.net_out_bps,
			disk_read_bps = excluded.disk_read_bps,
			disk_write_bps = excluded.disk_write_bps
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range samples {
		if _, err := stmt.ExecContext(ctx,
			m.AccountID, m.MachineID, m.TS, m.CPUPct, m.CPUCount, m.MemUsed, m.MemTotal,
			m.DiskUsed, m.DiskTotal, m.RootUsed, m.RootTotal,
			m.NetInBPS, m.NetOutBPS, m.DiskReadBPS, m.DiskWriteBPS,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetMachineMetrics(ctx context.Context, accountID, machineID string, sinceUnix int64) ([]MetricSample, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, cpu_pct, cpu_count, mem_used, mem_total,
		       disk_used, disk_total, root_used, root_total, net_in_bps, net_out_bps, disk_read_bps, disk_write_bps
		FROM machine_metrics
		WHERE account_id = ? AND machine_id = ? AND ts >= ?
		ORDER BY ts ASC
	`, accountID, machineID, sinceUnix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MetricSample, 0)
	for rows.Next() {
		var m MetricSample
		if err := rows.Scan(
			&m.TS, &m.CPUPct, &m.CPUCount, &m.MemUsed, &m.MemTotal,
			&m.DiskUsed, &m.DiskTotal, &m.RootUsed, &m.RootTotal,
			&m.NetInBPS, &m.NetOutBPS, &m.DiskReadBPS, &m.DiskWriteBPS,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) PruneMetricsBefore(ctx context.Context, cutoffUnix int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM machine_metrics WHERE ts < ?`, cutoffUnix)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
