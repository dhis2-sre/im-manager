package instance

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

var iv = []byte{83, 108, 97, 118, 97, 32, 85, 107, 114, 97, 105, 110, 105, 33, 33, 33}

func encryptText(text string, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	plainText := []byte(text)
	cfb := cipher.NewCFBEncrypter(block, iv)
	cipherText := make([]byte, len(plainText))
	cfb.XORKeyStream(cipherText, plainText)

	base64Encoded := base64.StdEncoding.EncodeToString(cipherText)

	return base64Encoded, nil
}

func decryptText(text string, key string) (string, error) {
	fmt.Println("Snot")
	cipherText, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	cfb := cipher.NewCFBDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	cfb.XORKeyStream(plainText, cipherText)

	return string(plainText), nil
}
