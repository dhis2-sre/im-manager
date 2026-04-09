package cluster

import (
	"fmt"

	filippoioage "filippo.io/age"
	"github.com/getsops/sops/v3"
	"github.com/getsops/sops/v3/aes"
	sops_age "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/keys"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	"github.com/getsops/sops/v3/stores/yaml"
	"github.com/getsops/sops/v3/version"
)

type Encryptor struct {
	KmsArn string
	AgeKey string
}

func NewEncryptor(kmsArn, ageKey string) (Encryptor, error) {
	if (kmsArn == "") == (ageKey == "") {
		return Encryptor{}, fmt.Errorf("exactly one of SOPS_KMS_ARN or SOPS_AGE_KEY must be set")
	}
	return Encryptor{KmsArn: kmsArn, AgeKey: ageKey}, nil
}

func (e Encryptor) keyGroups() ([]sops.KeyGroup, error) {
	var keyGroups []sops.KeyGroup

	if e.KmsArn != "" {
		masterKeys := kms.MasterKeysFromArnString(e.KmsArn, nil, "")
		var kmsMasterKeys []keys.MasterKey
		for _, k := range masterKeys {
			kmsMasterKeys = append(kmsMasterKeys, k)
		}
		keyGroups = append(keyGroups, kmsMasterKeys)
	}

	if e.AgeKey != "" {
		identity, err := filippoioage.ParseX25519Identity(e.AgeKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse age identity: %w", err)
		}
		ageKeys, err := sops_age.MasterKeysFromRecipients(identity.Recipient().String())
		if err != nil {
			return nil, fmt.Errorf("failed to get age master keys: %w", err)
		}
		var ageMasterKeys []keys.MasterKey
		for _, k := range ageKeys {
			ageMasterKeys = append(ageMasterKeys, k)
		}
		keyGroups = append(keyGroups, ageMasterKeys)
	}

	if len(keyGroups) == 0 {
		return nil, fmt.Errorf("no encryption key provided: set SOPS_KMS_ARN or SOPS_AGE_KEY")
	}

	return keyGroups, nil
}

func EncryptYaml(data []byte, keyGroups []sops.KeyGroup) ([]byte, error) {
	inputStore := &yaml.Store{}
	branches, err := inputStore.LoadPlainFile(data)
	if err != nil {
		return nil, fmt.Errorf("failed to load yaml: %w", err)
	}

	keyServices := []keyservice.KeyServiceClient{keyservice.NewLocalClient()}

	tree := sops.Tree{
		Branches: branches,
		Metadata: sops.Metadata{
			KeyGroups:         keyGroups,
			UnencryptedSuffix: "",
			EncryptedSuffix:   "",
			UnencryptedRegex:  "",
			EncryptedRegex:    "",
			Version:           version.Version,
			ShamirThreshold:   0,
		},
		FilePath: "",
	}

	dataKey, errs := tree.GenerateDataKeyWithKeyServices(keyServices)
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to generate data key: %v", errs)
	}

	err = common.EncryptTree(common.EncryptTreeOpts{
		DataKey: dataKey,
		Tree:    &tree,
		Cipher:  aes.NewCipher(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	outputStore := &yaml.Store{}
	encryptedFile, err := outputStore.EmitEncryptedFile(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to emit encrypted yaml: %w", err)
	}

	return encryptedFile, nil
}
