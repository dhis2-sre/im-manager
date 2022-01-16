package client

import (
	"github.com/dhis2-sre/im-manager/pkg/config"
	userClient "github.com/dhis2-sre/im-user/pkg/client"
)

func ProvideUserService(config config.Config) userClient.Client {
	return userClient.ProvideClient(config.UserService.Host, config.UserService.BasePath)
}
