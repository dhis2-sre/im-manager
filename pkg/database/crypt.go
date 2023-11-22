package database

import (
	"go.mozilla.org/sops/v3/cmd/sops/formats"
	"go.mozilla.org/sops/v3/decrypt"
)

func decryptYaml(data []byte) ([]byte, error) {
	return decrypt.DataWithFormat(data, formats.FormatFromString("yaml"))
}
