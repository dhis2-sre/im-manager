package instance

import (
	"strings"
	"testing"
)

const testKey = "12345678901234567890123456789012" // 32 bytes

func TestEncryptDecryptRoundtrip(t *testing.T) {
	plaintext := "super-secret-value"

	encrypted, err := encryptText(testKey, plaintext)
	if err != nil {
		t.Fatalf("encryptText: %v", err)
	}

	if !strings.HasPrefix(encrypted, gcmPrefix) {
		t.Fatalf("expected v2: prefix, got %q", encrypted)
	}

	decrypted, err := decryptText(testKey, encrypted)
	if err != nil {
		t.Fatalf("decryptText: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("want %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptProducesUniqueOutputs(t *testing.T) {
	plaintext := "same-value"

	a, err := encryptText(testKey, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	b, err := encryptText(testKey, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if a == b {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext")
	}
}

func TestDecryptLegacyCFB(t *testing.T) {
	// Ciphertext produced by the old AES-CFB + static IV implementation.
	// Plaintext: "legacy-secret", key: testKey
	legacy := "wjifidx8JVrxlz8JWg=="

	decrypted, err := decryptText(testKey, legacy)
	if err != nil {
		t.Fatalf("decryptText legacy: %v", err)
	}

	if decrypted != "legacy-secret" {
		t.Fatalf("want %q, got %q", "legacy-secret", decrypted)
	}
}
