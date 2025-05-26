package database

import (
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
)

func decryptYaml(data []byte) ([]byte, error) {
	return decrypt.DataWithFormat(data, formats.FormatFromString("yaml"))
}
