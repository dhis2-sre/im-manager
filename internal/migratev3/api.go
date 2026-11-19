package migratev3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

// API drives the instance manager the migrated deployments run under, so deploys go through its
// own deploy path, seeding included, rather than a copy of it.
type API struct {
	BaseURL  string
	Email    string
	Password string
	Client   *http.Client
	token    string
	issued   time.Time
}

// tokenLifetime stays well inside the access token expiration of every environment.
const tokenLifetime = 2 * time.Minute

func (a *API) authenticate(ctx context.Context) (string, error) {
	if a.token != "" && time.Since(a.issued) < tokenLifetime {
		return a.token, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+"/tokens", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	request.SetBasicAuth(a.Email, a.Password)
	request.Header.Set("Content-Type", "application/json")
	response, err := a.Client.Do(request)
	if err != nil {
		return "", fmt.Errorf("failed to sign in as %s: %v", a.Email, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("failed to sign in as %s: %v", a.Email, HTTPError{Status: response.StatusCode, Body: strings.TrimSpace(string(body))})
	}
	// Signing in answers with the tokens as cookies only.
	for _, cookie := range response.Cookies() {
		if cookie.Name == "accessToken" {
			a.token, a.issued = cookie.Value, time.Now()
			return a.token, nil
		}
	}
	return "", fmt.Errorf("signing in as %s returned no access token", a.Email)
}

func (a *API) do(ctx context.Context, method, path string, expected int, out any) error {
	token, err := a.authenticate(ctx)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, a.BaseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	return a.send(request, expected, out)
}

// HTTPError is a response with a status other than the expected one.
type HTTPError struct {
	Status int
	Body   string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("status %d: %s", e.Status, e.Body)
}

func (a *API) send(request *http.Request, expected int, out any) error {
	response, err := a.Client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return HTTPError{Status: response.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// Healthy reports whether an instance manager answers at the base url.
func (a *API) Healthy(ctx context.Context) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.BaseURL+"/health", nil)
	if err != nil {
		return false
	}
	return a.send(request, http.StatusOK, nil) == nil
}

// Deploy starts an asynchronous deploy of the deployment. A deployment that is already deploying
// counts as started, so a phase run again waits for the deploy it started before.
func (a *API) Deploy(ctx context.Context, deploymentID uint) error {
	err := a.do(ctx, http.MethodPost, fmt.Sprintf("/deployments/%d/deploy", deploymentID), http.StatusAccepted, nil)
	if httpErr, ok := err.(HTTPError); ok && httpErr.Status == http.StatusConflict {
		return nil
	}
	return err
}

func (a *API) Deployment(ctx context.Context, deploymentID uint) (*model.Deployment, error) {
	var deployment model.Deployment
	if err := a.do(ctx, http.MethodGet, fmt.Sprintf("/deployments/%d", deploymentID), http.StatusOK, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

func (a *API) Pause(ctx context.Context, instanceID uint) error {
	return a.do(ctx, http.MethodPut, fmt.Sprintf("/instances/%d/pause", instanceID), http.StatusAccepted, nil)
}

// DeleteDatabase deletes a database record together with its S3 object and file store.
func (a *API) DeleteDatabase(ctx context.Context, databaseID uint) error {
	err := a.do(ctx, http.MethodDelete, fmt.Sprintf("/databases/%d", databaseID), http.StatusAccepted, nil)
	if httpErr, ok := err.(HTTPError); ok && httpErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}
