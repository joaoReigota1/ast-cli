package wrappers

import (
	"fmt"
	"io/ioutil"
	"net/http"

	errors "github.com/pkg/errors"
)

type HealthCheckHTTPWrapper struct {
	webAppURL string
}

func NewHTTPHealthCheckWrapper(astWebAppURL string) HealthCheckWrapper {
	return &HealthCheckHTTPWrapper{webAppURL: astWebAppURL}
}

func (h *HealthCheckHTTPWrapper) RunWebAppCheck() (*HealthStatus, error) {
	resp, err := SendHTTPRequest(http.MethodGet, h.webAppURL, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Http request %v failed", h.webAppURL)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return &HealthStatus{
			Success: false,
			Message: fmt.Sprintf("HTTP status code %v with body %v", resp.StatusCode, func() string {
				if body != nil {
					return string(body)
				}

				return ""
			}()),
		}, nil
	}

	return &HealthStatus{Success: true, Message: ""}, nil
}