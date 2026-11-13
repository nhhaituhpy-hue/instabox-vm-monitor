package crypto_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
)

func TestLoadOrCreateMasterKeyPersistsAndReuses(t *testing.T) {
	dir := t.TempDir()
	key1, err := crypto.LoadOrCreateMasterKey(dir, "")
	if err != nil {
		t.Fatalf("LoadOrCreateMasterKey() err = %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}
	info, err := os.Stat(filepath.Join(dir, "master.key"))
	if err != nil {
		t.Fatalf("stat master.key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", info.Mode().Perm())
	}

	key2, err := crypto.LoadOrCreateMasterKey(dir, "")
	if err != nil {
		t.Fatalf("second LoadOrCreateMasterKey() err = %v", err)
	}
	if string(key1) != string(key2) {
		t.Fatalf("expected persisted master key to be reused")
	}

	envKey, err := crypto.LoadOrCreateMasterKey(dir, "my-explicit-master-secret-override")
	if err != nil {
		t.Fatalf("env override LoadOrCreateMasterKey() err = %v", err)
	}
	if len(envKey) != 32 || string(envKey) == string(key1) {
		t.Fatalf("expected env override to derive a distinct 32-byte key")
	}
}

func TestEncryptDecryptRoundTripAndTamperRejection(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	secret := "ibk_live_secret_token_998877"
	enc1, err := crypto.EncryptSecret(key, secret)
	if err != nil {
		t.Fatalf("EncryptSecret err = %v", err)
	}
	enc2, err := crypto.EncryptSecret(key, secret)
	if err != nil {
		t.Fatalf("EncryptSecret second err = %v", err)
	}
	if enc1 == enc2 {
		t.Fatalf("expected unique nonces on repeated encryption")
	}
	if strings.Contains(enc1, secret) {
		t.Fatalf("ciphertext must not contain plaintext")
	}

	dec, err := crypto.DecryptSecret(key, enc1)
	if err != nil {
		t.Fatalf("DecryptSecret err = %v", err)
	}
	if dec != secret {
		t.Fatalf("got %q, want %q", dec, secret)
	}

	// Tampered ciphertext must fail authentication.
	tampered := enc1[:len(enc1)-2] + "AA"
	if tampered == enc1 {
		tampered = enc1[:len(enc1)-2] + "BB"
	}
	if _, err := crypto.DecryptSecret(key, tampered); err == nil {
		t.Fatalf("expected tampered ciphertext to fail")
	}
}

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := crypto.HashPassword("S3cur3-Admin-Pass!")
	if err != nil {
		t.Fatalf("HashPassword err = %v", err)
	}
	if strings.Contains(hash, "S3cur3-Admin-Pass!") {
		t.Fatalf("hash must not contain plaintext")
	}
	if !crypto.VerifyPassword("S3cur3-Admin-Pass!", hash) {
		t.Fatalf("expected valid password to verify")
	}
	if crypto.VerifyPassword("wrong-password", hash) {
		t.Fatalf("expected invalid password to fail")
	}
	if crypto.VerifyPassword("S3cur3-Admin-Pass!", "malformed-hash") {
		t.Fatalf("expected malformed hash to fail")
	}
}

func TestMaskAPIKey(t *testing.T) {
	if got := crypto.MaskAPIKey("ibk_1234567890abcdef"); got != "ibk_12...cdef" {
		t.Fatalf("unexpected mask: %q", got)
	}
	if got := crypto.MaskAPIKey("short"); got != "****" {
		t.Fatalf("unexpected short mask: %q", got)
	}
}
