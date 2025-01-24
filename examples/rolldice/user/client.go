// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/auto/examples/rolldice/user/internal"
)

var (
	// ErrUnavailable is returned when the user service errors and is unavailable.
	ErrUnavailable = errors.New("user service unavailable")
	// ErrInsufficient is returned when the user does not have sufficient quota.
	ErrInsufficient = errors.New("insufficient quota")
)

// Client is a client to the user service API.
type Client struct {
	endpoint string
	client   *http.Client
}

// NewClient returns a new user service client. The client uses the provided
// HTTP client to connect to the user service at endpoint.
func NewClient(c *http.Client, endpoint string) *Client {
	endpoint = strings.TrimRight(endpoint, "/")
	return &Client{endpoint: endpoint, client: c}
}

func (c *Client) get(ctx context.Context, url, pattern string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Pattern = pattern
	return c.client.Do(req)
}

func (c *Client) checkStatus(resp *http.Response) error {
	switch {
	case resp.StatusCode >= http.StatusInternalServerError:
		return ErrUnavailable
	case resp.StatusCode >= http.StatusBadRequest:
		return fmt.Errorf("bad request: %s", http.StatusText(resp.StatusCode))
	default:
		return nil
	}
}

// HealthCheck checks the health of the user service. It will return an error
// if the service cannot be reached or is unhealthy, otherwise nil.
func (c *Client) HealthCheck(ctx context.Context) error {
	url := c.endpoint + "/healthcheck"
	resp, err := c.get(ctx, url, "/healthcheck")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.checkStatus(resp)
}

// UseQuota will allocate one use for the user with name. If the user no longer
// has any quota that can be allocated, ErrInsufficient will be returned.
func (c *Client) UseQuota(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/user/%s/alloc", c.endpoint, name)
	resp, err := c.get(ctx, url, "/user/{user}/alloc")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return err
	}

	var user internal.User
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return err
	}

	if user.Quota == 0 {
		return ErrInsufficient
	}
	return nil
}
