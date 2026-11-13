package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/scrypt"
)

const (
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32
	saltLen      = 16
)

// LoadOrCreateMasterKey returns a 32-byte key derived from override (if non-empty)
// or persisted at dataDir/master.key with 0600 permissions.
func LoadOrCreateMasterKey(dataDir, override string) ([]byte, error) {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		sum := sha256.Sum256([]byte("ibmon-master:" + trimmed))
		return sum[:], nil
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	keyPath := filepath.Join(dataDir, "master.key")
	raw, err := os.ReadFile(keyPath)
	if err == nil {
		decoded, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if decErr == nil && len(decoded) == 32 {
			return decoded, nil
		}
		return nil, errors.New("corrupt master.key file")
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read master.key: %w", err)
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key) + "\n"
	tmp := keyPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(encoded), 0o600); err != nil {
		return nil, fmt.Errorf("write master.key tmp: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("chmod master.key tmp: %w", err)
	}
	if err := os.Rename(tmp, keyPath); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("rename master.key: %w", err)
	}
	return key, nil
}

// EncryptSecret encrypts plaintext with AES-256-GCM using a fresh 12-byte nonce.
func EncryptSecret(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", errors.New("master key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "v1:" + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret decrypts ciphertext produced by EncryptSecret.
func DecryptSecret(key []byte, ciphertext string) (string, error) {
	if len(key) != 32 {
		return "", errors.New("master key must be 32 bytes")
	}
	raw, ok := strings.CutPrefix(ciphertext, "v1:")
	if !ok {
		return "", errors.New("unsupported ciphertext version")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, body := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// HashPassword derives a salted scrypt hash in format scrypt$saltB64$hashB64.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	dk, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scrypt$%s$%s",
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(dk)), nil
}

// VerifyPassword verifies a plaintext password against a HashPassword string.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "scrypt" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil || len(salt) != saltLen {
		return false
	}
	expected, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil || len(expected) != scryptKeyLen {
		return false
	}
	actual, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

// RandomToken returns a URL-safe random token and its SHA-256 hex digest.
func RandomToken(numBytes int) (token string, hashHex string, err error) {
	buf := make([]byte, numBytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

// HashToken returns SHA-256 hex digest of a raw token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewUUID generates a random RFC 4122 UUIDv4 string for Idempotency-Key and entity IDs.
func NewUUID() string {
	var b [16]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// MaskAPIKey returns a safe preview prefix+suffix without exposing the secret.
func MaskAPIKey(key string) string {
	k := strings.TrimSpace(key)
	if len(k) <= 10 {
		return "****"
	}
	return k[:6] + "..." + k[len(k)-4:]
}
