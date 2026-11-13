package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
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
