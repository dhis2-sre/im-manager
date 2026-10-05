package migratev3

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

// The parameter encryption of pkg/instance, which keeps its helpers unexported: AES-GCM behind a
// "v2:" prefix, and AES-CFB with a static IV for rows written before the GCM migration.
const gcmPrefix = "v2:"

func decryptInstance(key string, instance *model.DeploymentInstance, sensitive map[string]bool) error {
	for name, parameter := range instance.Parameters {
		if !sensitive[name] {
			continue
		}
		value, err := decryptText(key, parameter.Value)
		if err != nil {
			return fmt.Errorf("failed to decrypt %s parameter %s: %v", instance.StackName, name, err)
		}
		parameter.Value = value
		instance.Parameters[name] = parameter
	}
	return nil
}

func encryptText(key, text string) (string, error) {
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %v", err)
	}
	return gcmPrefix + base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(text), nil)), nil
}

func decryptText(key, text string) (string, error) {
	if !strings.HasPrefix(text, gcmPrefix) {
		return decryptCFB(key, text)
	}
	cipherText, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(text, gcmPrefix))
	if err != nil {
		return "", fmt.Errorf("failed to base64 decode: %v", err)
	}
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(cipherText) < aead.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, sealed := cipherText[:aead.NonceSize()], cipherText[aead.NonceSize():]
	plainText, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %v", err)
	}
	return string(plainText), nil
}

func decryptCFB(key, text string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	iv := []byte{83, 108, 97, 118, 97, 32, 85, 107, 114, 97, 105, 110, 105, 33, 33, 33}
	plainText := make([]byte, len(cipherText))
	cipher.NewCFBDecrypter(block, iv).XORKeyStream(plainText, cipherText) //nolint:staticcheck
	return string(plainText), nil
}

func newGCM(key string) (cipher.AEAD, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}
	return cipher.NewGCM(block)
}
