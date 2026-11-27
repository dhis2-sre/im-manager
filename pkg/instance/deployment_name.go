package instance

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// cnpgMaxClusterNameLength is the longest name CloudNativePG accepts for a cluster. Its webhook
// refuses anything longer, so the deploy that creates the cluster fails.
const cnpgMaxClusterNameLength = 50

// longestClusterSuffix follows the instance name in the name of the longest cluster a stack creates,
// the dhis2-v2 database <name>-<groupID>-dhis2-postgresql. It is the tightest of the limits on names
// derived from an instance name: every other resource name, label value and helm release name a
// stack derives leaves more room under its own limit.
const longestClusterSuffix = "-dhis2-postgresql"

// maxDeploymentNameLength is the longest name a deployment in the group can have and still deploy.
// The stacks derive their resource names from the instance name, which carries the group's id, so
// a group with a longer id leaves less room for the name.
func maxDeploymentNameLength(group *model.Group) int {
	return cnpgMaxClusterNameLength - len(longestClusterSuffix) - len(instanceName(&model.DeploymentInstance{}, group))
}

// deploymentNameReason says why the name can't be given to a new deployment in the group, or
// returns an empty string when it can. Existing deployments are not held to it, since they were
// named before the limit was enforced.
func deploymentNameReason(name string, group *model.Group) string {
	if maxLength := maxDeploymentNameLength(group); len(name) > maxLength {
		return fmt.Sprintf("name must be at most %d characters in group %q", maxLength, group.Name)
	}
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok && v.Var(name, "dns_rfc1035_label") != nil {
		return "name must consist of lowercase letters, digits and '-', start with a letter and end with a letter or digit"
	}
	return ""
}
