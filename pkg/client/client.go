package client

import (
	"context"

	"github.com/dhis2-sre/im-manager/swagger/sdk/client/operations"
	"github.com/dhis2-sre/im-manager/swagger/sdk/models"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

type instanceClient struct {
	client operations.ClientService
}

func New(host string, basePath string) *instanceClient {
	transport := httptransport.New(host, basePath, nil)
	return &instanceClient{
		client: operations.New(transport, strfmt.Default),
	}
}

func (c instanceClient) FindByIdDecrypted(token string, id uint) (*models.Instance, error) {
	params := &operations.FindByIDDecryptedParams{ID: uint64(id), Context: context.Background()}
	clientAuthInfoWriter := httptransport.BearerToken(token)
	instance, err := c.client.FindByIDDecrypted(params, clientAuthInfoWriter)
	if err != nil {
		return nil, err
	}
	return instance.GetPayload(), nil
}
