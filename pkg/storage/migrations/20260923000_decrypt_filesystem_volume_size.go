package migrations

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// decryptFilesystemVolumeSize undoes the encryption of a parameter that was never a secret. A volume
// size was declared sensitive by mistake, so the stored values are ciphertext while nothing decrypts
// them any more: without this the instances that use filesystem storage would render and deploy their
// volume size as the base64 of its ciphertext.
func decryptFilesystemVolumeSize() *gormigrate.Migration {
	const (
		gcmPrefix     = "v2:"
		stackName     = "dhis2-v2"
		parameterName = "FILESYSTEM_VOLUME_SIZE"
	)

	gcm := func(key string) (cipher.AEAD, error) {
		block, err := aes.NewCipher([]byte(key))
		if err != nil {
			return nil, err
		}
		return cipher.NewGCM(block)
	}

	decrypt := func(key, value string) (string, error) {
		aead, err := gcm(key)
		if err != nil {
			return "", err
		}
		cipherText, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, gcmPrefix))
		if err != nil {
			return "", err
		}
		nonceSize := aead.NonceSize()
		if len(cipherText) < nonceSize {
			return "", fmt.Errorf("ciphertext too short")
		}
		plainText, err := aead.Open(nil, cipherText[:nonceSize], cipherText[nonceSize:], nil)
		if err != nil {
			return "", err
		}
		return string(plainText), nil
	}

	encrypt := func(key, value string) (string, error) {
		aead, err := gcm(key)
		if err != nil {
			return "", err
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return "", err
		}
		return gcmPrefix + base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(value), nil)), nil
	}

	convert := func(tx *gorm.DB, to func(key, value string) (string, error), onlyEncrypted bool) error {
		key := os.Getenv("INSTANCE_PARAMETER_ENCRYPTION_KEY")
		if key == "" {
			return fmt.Errorf("INSTANCE_PARAMETER_ENCRYPTION_KEY is not set")
		}

		var parameters []model.DeploymentInstanceParameter
		err := tx.Where("stack_name = ? AND parameter_name = ?", stackName, parameterName).Find(&parameters).Error
		if err != nil {
			return fmt.Errorf("failed to load %s parameters: %w", parameterName, err)
		}

		for _, parameter := range parameters {
			if onlyEncrypted != strings.HasPrefix(parameter.Value, gcmPrefix) {
				continue
			}

			value, err := to(key, parameter.Value)
			if err != nil {
				return fmt.Errorf("failed to convert %s on instance %d: %w", parameterName, parameter.DeploymentInstanceID, err)
			}

			err = tx.Model(&model.DeploymentInstanceParameter{}).
				Where("deployment_instance_id = ? AND parameter_name = ?", parameter.DeploymentInstanceID, parameterName).
				Update("value", value).Error
			if err != nil {
				return fmt.Errorf("failed to save %s on instance %d: %w", parameterName, parameter.DeploymentInstanceID, err)
			}
		}
		return nil
	}

	return &gormigrate.Migration{
		ID: "20260923000",
		Migrate: func(tx *gorm.DB) error {
			return convert(tx, decrypt, true)
		},
		Rollback: func(tx *gorm.DB) error {
			return convert(tx, encrypt, false)
		},
	}
}
