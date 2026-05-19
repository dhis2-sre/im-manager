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
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func reencryptCFBToGCM() *gormigrate.Migration {
	const gcmPrefix = "v2:"

	allStacks := []model.Stack{
		stack.DHIS2DB,
		stack.MINIO,
		stack.DHIS2Core,
		stack.DHIS2,
		stack.PgAdmin,
		stack.WhoamiGo,
		stack.IMJobRunner,
		stack.ChapDB,
		stack.ChapValkey,
		stack.ChapWorker,
		stack.ChapCore,
	}

	sensitive := map[string]map[string]bool{}
	for _, s := range allStacks {
		m := map[string]bool{}
		for name, p := range s.Parameters {
			if p.Sensitive {
				m[name] = true
			}
		}
		sensitive[s.Name] = m
	}

	encryptGCM := func(key, plaintext string) (string, error) {
		block, err := aes.NewCipher([]byte(key))
		if err != nil {
			return "", err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
			return "", err
		}
		return gcmPrefix + base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
	}

	decryptCFB := func(key, encoded string) (string, error) {
		ct, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", err
		}
		block, err := aes.NewCipher([]byte(key))
		if err != nil {
			return "", err
		}
		iv := []byte{83, 108, 97, 118, 97, 32, 85, 107, 114, 97, 105, 110, 105, 33, 33, 33}
		pt := make([]byte, len(ct))
		cipher.NewCFBDecrypter(block, iv).XORKeyStream(pt, ct) //nolint:staticcheck
		return string(pt), nil
	}

	decryptGCM := func(key, encoded string) (string, error) {
		ct, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", err
		}
		block, err := aes.NewCipher([]byte(key))
		if err != nil {
			return "", err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}
		ns := gcm.NonceSize()
		if len(ct) < ns {
			return "", fmt.Errorf("ciphertext too short")
		}
		pt, err := gcm.Open(nil, ct[:ns], ct[ns:], nil)
		if err != nil {
			return "", err
		}
		return string(pt), nil
	}

	encryptCFB := func(key, plaintext string) (string, error) {
		block, err := aes.NewCipher([]byte(key))
		if err != nil {
			return "", err
		}
		iv := []byte{83, 108, 97, 118, 97, 32, 85, 107, 114, 97, 105, 110, 105, 33, 33, 33}
		ct := make([]byte, len(plaintext))
		cipher.NewCFBEncrypter(block, iv).XORKeyStream(ct, []byte(plaintext)) //nolint:staticcheck
		return base64.StdEncoding.EncodeToString(ct), nil
	}

	reencrypt := func(tx *gorm.DB, fromEncrypted, toEncrypted func(key, val string) (string, error), skipPrefix string) error {
		key := os.Getenv("INSTANCE_PARAMETER_ENCRYPTION_KEY")
		if key == "" {
			return fmt.Errorf("INSTANCE_PARAMETER_ENCRYPTION_KEY is not set")
		}

		var params []model.DeploymentInstanceParameter
		if err := tx.Find(&params).Error; err != nil {
			return fmt.Errorf("failed to load instance parameters: %w", err)
		}

		for _, param := range params {
			if !sensitive[param.StackName][param.ParameterName] {
				continue
			}
			if skipPrefix != "" && !strings.HasPrefix(param.Value, skipPrefix) {
				continue
			}
			value := param.Value
			if skipPrefix != "" {
				value = value[len(skipPrefix):]
			}

			plaintext, err := fromEncrypted(key, value)
			if err != nil {
				return fmt.Errorf("failed to decrypt parameter %q on instance %d: %w", param.ParameterName, param.DeploymentInstanceID, err)
			}

			encrypted, err := toEncrypted(key, plaintext)
			if err != nil {
				return fmt.Errorf("failed to re-encrypt parameter %q on instance %d: %w", param.ParameterName, param.DeploymentInstanceID, err)
			}

			err = tx.Model(&param).
				Where("deployment_instance_id = ? AND parameter_name = ?", param.DeploymentInstanceID, param.ParameterName).
				Update("value", encrypted).Error
			if err != nil {
				return fmt.Errorf("failed to save re-encrypted parameter %q on instance %d: %w", param.ParameterName, param.DeploymentInstanceID, err)
			}
		}
		return nil
	}

	return &gormigrate.Migration{
		ID: "20260515000",
		Migrate: func(tx *gorm.DB) error {
			return reencrypt(tx, decryptCFB, encryptGCM, "")
		},
		Rollback: func(tx *gorm.DB) error {
			return reencrypt(tx, decryptGCM, encryptCFB, gcmPrefix)
		},
	}
}
