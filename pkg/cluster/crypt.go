package cluster

import (
	"fmt"

	"github.com/getsops/sops/v3"
	"github.com/getsops/sops/v3/aes"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/keys"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	"github.com/getsops/sops/v3/stores/yaml"
	"github.com/getsops/sops/v3/version"
)

func encryptYaml(data []byte, kmsKey string) ([]byte, error) {
	if kmsKey == "" {
		return nil, fmt.Errorf("empty key group provided")
	}

	masterKeys := kms.MasterKeysFromArnString(kmsKey, nil, "")
	var kmsMasterKeys []keys.MasterKey
	for _, k := range masterKeys {
		kmsMasterKeys = append(kmsMasterKeys, k)
	}
	keyGroups := []sops.KeyGroup{kmsMasterKeys}

	return EncryptYaml(data, keyGroups)
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
