package client

import (
	jobClient "github.com/dhis2-sre/im-job/pkg/client"
	"github.com/dhis2-sre/im-manager/pkg/config"
)

func ProvideJobService(config config.Config) jobClient.Client {
	return jobClient.ProvideClient(config.JobService.Host, config.JobService.BasePath)
}
