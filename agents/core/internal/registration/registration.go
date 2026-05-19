package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"example.com/rmm-shared/api"
)

// Client handles agent registration against the registration-service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Enrol registers this agent with the platform and returns the assigned
// AgentID and BrokerURL. Call once at startup before connecting transport.
func (c *Client) Enrol(ctx context.Context, tenantID, enrollmentToken, hostname string) (*api.RegistrationResponse, error) {
	reqBody := api.RegistrationRequest{
		TenantID:        tenantID,
		EnrollmentToken: enrollmentToken,
		Hostname:        hostname,
		OSFamily:        runtime.GOOS,
		OSVersion:       runtime.GOOS + "/" + runtime.GOARCH,
		Architecture:    runtime.GOARCH,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal registration request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/agents/register",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("build registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("registration rejected (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var regResp api.RegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, fmt.Errorf("decode registration response: %w", err)
	}

	if regResp.AgentID == "" || regResp.BrokerURL == "" {
		return nil, fmt.Errorf("registration response missing agent_id or broker_url")
	}

	return &regResp, nil
}
