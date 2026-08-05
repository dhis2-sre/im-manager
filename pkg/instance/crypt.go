package instance

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const gcmPrefix = "v2:"

func encryptText(key string, text string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %v", err)
	}

	cipherText := gcm.Seal(nonce, nonce, []byte(text), nil)
	return gcmPrefix + base64.StdEncoding.EncodeToString(cipherText), nil
}

func decryptText(key string, text string) (string, error) {
	if strings.HasPrefix(text, gcmPrefix) {
		return decryptGCM(key, text[len(gcmPrefix):])
	}
	return decryptCFB(key, text)
}

func decryptGCM(key string, encoded string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to base64 decode: %v", err)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %v", err)
	}

	return string(plainText), nil
}

// decryptCFB decrypts legacy AES-CFB ciphertext with the static IV.
// Only used for backward compatibility with existing database rows.
func decryptCFB(key string, encoded string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	iv := []byte{83, 108, 97, 118, 97, 32, 85, 107, 114, 97, 105, 110, 105, 33, 33, 33}
	cfb := cipher.NewCFBDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	cfb.XORKeyStream(plainText, cipherText)

	return string(plainText), nil
}
