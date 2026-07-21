package instance

import (
	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// NewKubernetesService is a temporary bridge to kube.NewClient that lets existing callers compile
// unchanged during the pkg/kube extraction. Prefer kube.NewClient directly; this is removed in the
// per-stack component operations change once the remaining callers move over.
func NewKubernetesService(c model.Cluster) (*kube.Client, error) {
	return kube.NewClient(c)
}
