package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) (*store.Store, []byte) {
	t.Helper()
	dir := t.TempDir()
	key, err := crypto.LoadOrCreateMasterKey(dir, "")
	if err != nil {
		t.Fatalf("master key: %v", err)
	}
	st, err := store.Open(filepath.Join(dir, "monitor.db"), key)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, key
}

func TestAdminSingleInitialSetupAndSessionLifecycle(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()

	has, err := st.HasAdmin(ctx)
	if err != nil || has {
		t.Fatalf("expected no admin initially, got has=%v err=%v", has, err)
	}

	if err := st.CreateInitialAdmin(ctx, "admin", "StrongPass!123"); err != nil {
		t.Fatalf("CreateInitialAdmin: %v", err)
	}

	has, err = st.HasAdmin(ctx)
	if err != nil || !has {
		t.Fatalf("expected admin to exist, has=%v err=%v", has, err)
	}

	// Second setup attempt MUST fail.
	if err := st.CreateInitialAdmin(ctx, "other", "AnotherPass!123"); !errors.Is(err, store.ErrAdminExists) {
		t.Fatalf("expected ErrAdminExists, got %v", err)
	}

	if _, err := st.AuthenticateAdmin(ctx, "admin", "wrong"); !errors.Is(err, store.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials on bad password, got %v", err)
	}

	admin, err := st.AuthenticateAdmin(ctx, "admin", "StrongPass!123")
	if err != nil || admin.Username != "admin" {
		t.Fatalf("AuthenticateAdmin failed: %v", err)
	}

	tok, sess, err := st.CreateSession(ctx, admin.ID, time.Hour)
	if err != nil || tok == "" || sess.CSRFToken == "" {
		t.Fatalf("CreateSession failed: err=%v", err)
	}

	loaded, err := st.GetSession(ctx, tok)
	if err != nil || loaded.AdminID != admin.ID || loaded.CSRFToken != sess.CSRFToken {
		t.Fatalf("GetSession failed: %v", err)
	}

	if err := st.DeleteSession(ctx, tok); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := st.GetSession(ctx, tok); !errors.Is(err, store.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestAccountsEncryptedKeyStorageAndMachinesMetricsCascade(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()

	acc, err := st.CreateAccount(ctx, store.AccountInput{
		Alias:          "Box 1",
		APIKey:         "ibk_secret_live_1234567890",
		InstaboxUserID: 42,
		InstaboxLogin:  "hainh",
		OrgID:          "org_personal_1",
		OrgName:        "hainh",
		OrgTier:        "free",
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if acc.KeyMask != "ibk_se...7890" {
		t.Fatalf("unexpected mask: %q", acc.KeyMask)
	}

	plainKey, err := st.GetAccountAPIKey(ctx, acc.ID)
	if err != nil || plainKey != "ibk_secret_live_1234567890" {
		t.Fatalf("GetAccountAPIKey mismatch: %v", err)
	}

	if err := st.UpdateAccount(ctx, acc.ID, "Box 1 Main", "", nil); err != nil {
		t.Fatalf("UpdateAccount alias: %v", err)
	}
	list, err := st.ListAccounts(ctx)
	if err != nil || len(list) != 1 || list[0].Alias != "Box 1 Main" {
		t.Fatalf("ListAccounts unexpected: %+v err=%v", list, err)
	}

	now := time.Now().Unix()
	err = st.SyncAccountMachines(ctx, acc.ID, []store.MachineRecord{
		{
			AccountID:             acc.ID,
			MachineID:             "m_vm1",
			Name:                  "instabox-ci",
			State:                 "running",
			DesiredState:          "running",
			VCPUs:                 4,
			MemMiB:                8192,
			PersistentDiskGiB:     20,
			RootDiskGiB:           16,
			TemplateID:            "ci",
			Free:                  true,
			Ready:                 true,
			MaxRunMinutes:         480,
			MonthlyRunningSeconds: 3660,
		},
	})
	if err != nil {
		t.Fatalf("SyncAccountMachines: %v", err)
	}

	err = st.UpsertMetrics(ctx, []store.MetricSample{
		{AccountID: acc.ID, MachineID: "m_vm1", TS: now - 3600, CPUPct: 12.5, CPUCount: 4, MemUsed: 2048 << 20, MemTotal: 8192 << 20, DiskUsed: 4 << 30, DiskTotal: 20 << 30},
		{AccountID: acc.ID, MachineID: "m_vm1", TS: now - 30, CPUPct: 44.2, CPUCount: 4, MemUsed: 4096 << 20, MemTotal: 8192 << 20, DiskUsed: 5 << 30, DiskTotal: 20 << 30},
		{AccountID: acc.ID, MachineID: "m_vm1", TS: now - 10*86400, CPUPct: 99.0, CPUCount: 4, MemUsed: 1 << 30, MemTotal: 8192 << 20, DiskUsed: 1 << 30, DiskTotal: 20 << 30},
	})
	if err != nil {
		t.Fatalf("UpsertMetrics: %v", err)
	}

	deleted, err := st.PruneMetricsBefore(ctx, now-7*86400)
	if err != nil || deleted != 1 {
		t.Fatalf("expected 1 pruned sample, got deleted=%d err=%v", deleted, err)
	}

	samples, err := st.GetMachineMetrics(ctx, acc.ID, "m_vm1", now-7200)
	if err != nil || len(samples) != 2 || samples[1].CPUPct != 44.2 {
		t.Fatalf("GetMachineMetrics unexpected: %+v err=%v", samples, err)
	}

	listed, err := st.ListMachines(ctx)
	if err != nil || len(listed) != 1 || len(listed[0].RecentMetrics) != 2 || listed[0].LatestMetric == nil {
		t.Fatalf("expected RecentMetrics and LatestMetric on ListMachines, got %+v err=%v", listed, err)
	}

	// Deleting account cascades to machines & metrics.
	if err := st.DeleteAccount(ctx, acc.ID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	machines, err := st.ListMachines(ctx)
	if err != nil || len(machines) != 0 {
		t.Fatalf("expected 0 machines after delete, got %d err=%v", len(machines), err)
	}
	samplesAfter, err := st.GetMachineMetrics(ctx, acc.ID, "m_vm1", 0)
	if err != nil || len(samplesAfter) != 0 {
		t.Fatalf("expected 0 samples after cascade, got %d err=%v", len(samplesAfter), err)
	}
}

func TestMachineKeepalivePersistenceAndLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")
	key, err := crypto.LoadOrCreateMasterKey(dir, "")
	if err != nil {
		t.Fatalf("master key: %v", err)
	}

	// Simulate an existing production database before keepalive columns were added.
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy sqlite: %v", err)
	}
	_, err = rawDB.Exec(`
		CREATE TABLE accounts (
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
		CREATE TABLE machines (
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
		INSERT INTO accounts(id, alias, key_enc, key_mask, created_at, updated_at) VALUES ('acc_old', 'Legacy', 'enc', 'ibk_****1234', 100, 100);
		INSERT INTO machines(account_id, machine_id, name, state, desired_state) VALUES ('acc_old', 'm_old', 'legacy-box', 'running', 'running');
	`)
	_ = rawDB.Close()
	if err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	st, err := store.Open(dbPath, key)
	if err != nil {
		t.Fatalf("store.Open legacy db: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	m, err := st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || m.KeepaliveEnabled {
		t.Fatalf("expected legacy machine migrated with keepalive=false, got err=%v m=%+v", err, m)
	}

	if !errors.Is(st.SetMachineKeepalive(ctx, "acc_old", "missing", true), store.ErrMachineNotFound) {
		t.Fatalf("expected ErrMachineNotFound for missing machine")
	}

	if err := st.SetMachineKeepalive(ctx, "acc_old", "m_old", true); err != nil {
		t.Fatalf("SetMachineKeepalive(true): %v", err)
	}
	if err := st.UpdateMachineKeepaliveState(ctx, "acc_old", "m_old", 1794624000, 1794620400, "lease_renewed", ""); err != nil {
		t.Fatalf("UpdateMachineKeepaliveState: %v", err)
	}

	// SyncAccountMachines must NOT reset keepalive settings or status.
	err = st.SyncAccountMachines(ctx, "acc_old", []store.MachineRecord{
		{AccountID: "acc_old", MachineID: "m_old", Name: "legacy-box", State: "running", DesiredState: "running"},
	})
	if err != nil {
		t.Fatalf("SyncAccountMachines: %v", err)
	}

	m, err = st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || !m.KeepaliveEnabled || m.KeepaliveLeaseExpiresAt != 1794624000 || m.KeepaliveLastAction != "lease_renewed" {
		t.Fatalf("expected keepalive preserved across sync, got err=%v m=%+v", err, m)
	}

	if err := st.SetMachineKeepalive(ctx, "acc_old", "m_old", false); err != nil {
		t.Fatalf("SetMachineKeepalive(false): %v", err)
	}
	m, err = st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || m.KeepaliveEnabled || m.KeepaliveLeaseExpiresAt != 0 || m.KeepaliveLastAction != "" {
		t.Fatalf("expected keepalive cleared after disable, got err=%v m=%+v", err, m)
	}

	// UpdateMachineKeepaliveState MUST be fenced when keepalive_enabled=0.
	if err := st.UpdateMachineKeepaliveState(ctx, "acc_old", "m_old", 1794699999, 1794690000, "lease_renewed", "stale_err"); err != nil {
		t.Fatalf("UpdateMachineKeepaliveState when disabled: %v", err)
	}
	m, err = st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || m.KeepaliveEnabled || m.KeepaliveLeaseExpiresAt != 0 || m.KeepaliveLastAt != 0 || m.KeepaliveLastAction != "" || m.KeepaliveLastError != "" {
		t.Fatalf("expected UpdateMachineKeepaliveState ignored while disabled, got err=%v m=%+v", err, m)
	}

	// SetMachineKeepalivePendingRestart MUST be fenced when keepalive_enabled=0.
	if err := st.SetMachineKeepalivePendingRestart(ctx, "acc_old", "m_old", true); err != nil {
		t.Fatalf("SetMachineKeepalivePendingRestart(true) when disabled: %v", err)
	}
	m, err = st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || m.KeepalivePendingRestart {
		t.Fatalf("expected pending restart ignored while disabled, got err=%v m=%+v", err, m)
	}

	// Enable keepalive, set pending restart, verify preserved across SyncAccountMachines, cleared by false and disable.
	if err := st.SetMachineKeepalive(ctx, "acc_old", "m_old", true); err != nil {
		t.Fatalf("SetMachineKeepalive(true): %v", err)
	}
	if err := st.SetMachineKeepalivePendingRestart(ctx, "acc_old", "m_old", true); err != nil {
		t.Fatalf("SetMachineKeepalivePendingRestart(true): %v", err)
	}
	err = st.SyncAccountMachines(ctx, "acc_old", []store.MachineRecord{
		{AccountID: "acc_old", MachineID: "m_old", Name: "legacy-box", State: "stopped", DesiredState: "stopped"},
	})
	if err != nil {
		t.Fatalf("SyncAccountMachines: %v", err)
	}
	m, err = st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || !m.KeepalivePendingRestart {
		t.Fatalf("expected keepalive_pending_restart preserved across sync, got err=%v m=%+v", err, m)
	}
	if err := st.SetMachineKeepalivePendingRestart(ctx, "acc_old", "m_old", false); err != nil {
		t.Fatalf("SetMachineKeepalivePendingRestart(false): %v", err)
	}
	m, err = st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || m.KeepalivePendingRestart {
		t.Fatalf("expected keepalive_pending_restart cleared, got err=%v m=%+v", err, m)
	}

	// SetMachineKeepalive(false) must also clear pending restart.
	if err := st.SetMachineKeepalivePendingRestart(ctx, "acc_old", "m_old", true); err != nil {
		t.Fatalf("SetMachineKeepalivePendingRestart(true) again: %v", err)
	}
	if err := st.SetMachineKeepalive(ctx, "acc_old", "m_old", false); err != nil {
		t.Fatalf("SetMachineKeepalive(false): %v", err)
	}
	m, err = st.GetMachine(ctx, "acc_old", "m_old")
	if err != nil || m.KeepaliveEnabled || m.KeepalivePendingRestart || m.KeepaliveLeaseExpiresAt != 0 || m.KeepaliveLastAction != "" {
		t.Fatalf("expected keepalive cleared after disable, got err=%v m=%+v", err, m)
	}
}

func TestAPIKeyLifecycleAndVerification(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()

	if _, _, err := st.CreateAPIKey(ctx, ""); err == nil {
		t.Fatalf("expected error on empty api key name")
	}

	raw, rec, err := st.CreateAPIKey(ctx, "Hermes Agent")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if !strings.HasPrefix(raw, "ibmon_") || len(raw) < 24 {
		t.Fatalf("unexpected raw key format: %q", raw)
	}
	if rec.ID == "" || rec.Name != "Hermes Agent" || rec.KeyMask == "" || strings.Contains(rec.KeyMask, raw[10:]) {
		t.Fatalf("unexpected api key record or key leak in mask: %+v", rec)
	}

	list, err := st.ListAPIKeys(ctx)
	if err != nil || len(list) != 1 || list[0].ID != rec.ID {
		t.Fatalf("ListAPIKeys unexpected: err=%v list=%+v", err, list)
	}

	authRec, err := st.AuthenticateAPIKey(ctx, raw)
	if err != nil || authRec.ID != rec.ID || authRec.Name != "Hermes Agent" {
		t.Fatalf("AuthenticateAPIKey failed: err=%v rec=%+v", err, authRec)
	}
	if _, err := st.AuthenticateAPIKey(ctx, "ibmon_wrong_token"); !errors.Is(err, store.ErrAPIKeyNotFound) {
		t.Fatalf("expected ErrAPIKeyNotFound on wrong token, got %v", err)
	}

	list, _ = st.ListAPIKeys(ctx)
	if list[0].LastUsedAt == 0 {
		t.Fatalf("expected LastUsedAt updated after authentication")
	}

	if err := st.DeleteAPIKey(ctx, rec.ID); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
	if !errors.Is(st.DeleteAPIKey(ctx, rec.ID), store.ErrAPIKeyNotFound) {
		t.Fatalf("expected ErrAPIKeyNotFound on repeat delete")
	}
	if _, err := st.AuthenticateAPIKey(ctx, raw); !errors.Is(err, store.ErrAPIKeyNotFound) {
		t.Fatalf("expected revoked key to fail auth, got %v", err)
	}
}

func TestListMachinesPrioritizesRunningFirst(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()

	acc1, err := st.CreateAccount(ctx, store.AccountInput{Alias: "Box 1", APIKey: "ibk_acc1_1234567890"})
	if err != nil {
		t.Fatalf("CreateAccount 1: %v", err)
	}
	acc2, err := st.CreateAccount(ctx, store.AccountInput{Alias: "Box 2", APIKey: "ibk_acc2_1234567890"})
	if err != nil {
		t.Fatalf("CreateAccount 2: %v", err)
	}

	if err := st.SyncAccountMachines(ctx, acc1.ID, []store.MachineRecord{
		{AccountID: acc1.ID, MachineID: "m_stop_a", Name: "alpha-stopped", State: "stopped", DesiredState: "stopped"},
		{AccountID: acc1.ID, MachineID: "m_susp_a", Name: "bravo-paused", State: "suspended", DesiredState: "suspended"},
		{AccountID: acc1.ID, MachineID: "m_run_a", Name: "zulu-running", State: "running", DesiredState: "running"},
	}); err != nil {
		t.Fatalf("SyncAccountMachines 1: %v", err)
	}
	if err := st.SyncAccountMachines(ctx, acc2.ID, []store.MachineRecord{
		{AccountID: acc2.ID, MachineID: "m_err_b", Name: "charlie-error", State: "error", DesiredState: "running"},
		{AccountID: acc2.ID, MachineID: "m_boot_b", Name: "delta-booting", State: "starting", DesiredState: "running"},
		{AccountID: acc2.ID, MachineID: "m_run_b", Name: "alpha-running", State: "running", DesiredState: "running"},
	}); err != nil {
		t.Fatalf("SyncAccountMachines 2: %v", err)
	}

	listed, err := st.ListMachines(ctx)
	if err != nil {
		t.Fatalf("ListMachines: %v", err)
	}
	// Running machines must always come first, followed by booting, suspended, stopped, error.
	if len(listed) != 6 {
		t.Fatalf("expected 6 machines, got %d", len(listed))
	}
	gotStates := make([]string, len(listed))
	for i, m := range listed {
		gotStates[i] = m.State
	}
	wantStates := []string{"running", "running", "starting", "suspended", "stopped", "error"}
	for i, w := range wantStates {
		if gotStates[i] != w {
			t.Fatalf("state order mismatch at %d: got %v, want %v", i, gotStates, wantStates)
		}
	}
	if listed[0].MachineID != "m_run_b" || listed[1].MachineID != "m_run_a" {
		t.Fatalf("expected running machines sorted deterministically by name when created_at ties, got %s then %s", listed[0].MachineID, listed[1].MachineID)
	}
}
