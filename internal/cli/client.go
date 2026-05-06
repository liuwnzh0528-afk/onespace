package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wnzhone/onespace/internal/health"
	"github.com/wnzhone/onespace/internal/serviceops"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type ServiceSummary struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Image    string `json:"image"`
	Branch   string `json:"branch,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Health   string `json:"health,omitempty"`
}

type ServiceConfig struct {
	Service   string          `json:"service"`
	Env       []ConfigEnv     `json:"env"`
	Files     []ConfigFile    `json:"files"`
	Volumes   []ConfigVolume  `json:"volumes"`
	DependsOn []string        `json:"dependsOn"`
	Warnings  []ConfigWarning `json:"warnings,omitempty"`
}

type ConfigEnv struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"`
	Secret bool   `json:"secret"`
}

type ConfigFile struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
	Secret bool   `json:"secret"`
}

type ConfigVolume struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type ConfigWarning struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c Client) GetServices(ctx context.Context) ([]ServiceSummary, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/services", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var services []ServiceSummary
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, err
	}
	return services, nil
}

func (c Client) Config(ctx context.Context, service string) (ServiceConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/services/"+service+"/config", nil)
	if err != nil {
		return ServiceConfig{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ServiceConfig{}, err
	}
	defer resp.Body.Close()

	var cfg ServiceConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return ServiceConfig{}, err
	}
	if resp.StatusCode >= 400 {
		return cfg, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return cfg, nil
}

func (c Client) Deploy(ctx context.Context, service string, wait bool) (serviceops.Result, error) {
	url := fmt.Sprintf("%s/api/services/%s/deploy", c.BaseURL, service)
	return c.postServiceAction(ctx, url)
}

func (c Client) Debug(ctx context.Context, service string, wait bool) (serviceops.Result, error) {
	url := fmt.Sprintf("%s/api/services/%s/debug", c.BaseURL, service)
	return c.postServiceAction(ctx, url)
}

func (c Client) postServiceAction(ctx context.Context, url string) (serviceops.Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return serviceops.Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return serviceops.Result{}, err
	}
	defer resp.Body.Close()

	var result serviceops.Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return serviceops.Result{}, err
	}
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return result, nil
}

func (c Client) Logs(ctx context.Context, service string, jobID string, tail int) ([]string, error) {
	url := fmt.Sprintf("%s/api/jobs/%s/logs?tail=%d", c.BaseURL, jobID, tail)
	if jobID == "" {
		url = fmt.Sprintf("%s/api/services/%s/logs?tail=%d", c.BaseURL, service, tail)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Lines []string `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Lines, nil
}

func (c Client) Health(ctx context.Context, service string) (health.Result, error) {
	url := fmt.Sprintf("%s/api/services/%s/health", c.BaseURL, service)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return health.Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return health.Result{}, err
	}
	defer resp.Body.Close()

	var result health.Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return health.Result{}, err
	}
	return result, nil
}

func (c Client) Pull(ctx context.Context, service string) (serviceops.Result, error) {
	url := fmt.Sprintf("%s/api/services/%s/pull", c.BaseURL, service)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return serviceops.Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return serviceops.Result{}, err
	}
	defer resp.Body.Close()

	var result serviceops.Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return serviceops.Result{}, err
	}
	return result, nil
}

func (c Client) Build(ctx context.Context, service string) (serviceops.Result, error) {
	url := fmt.Sprintf("%s/api/services/%s/build", c.BaseURL, service)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return serviceops.Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return serviceops.Result{}, err
	}
	defer resp.Body.Close()

	var result serviceops.Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return serviceops.Result{}, err
	}
	return result, nil
}

func (c Client) Restart(ctx context.Context, service string) (serviceops.Result, error) {
	url := fmt.Sprintf("%s/api/services/%s/restart", c.BaseURL, service)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return serviceops.Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return serviceops.Result{}, err
	}
	defer resp.Body.Close()

	var result serviceops.Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return serviceops.Result{}, err
	}
	return result, nil
}

func (c Client) Stop(ctx context.Context, service string) (serviceops.Result, error) {
	url := fmt.Sprintf("%s/api/services/%s/stop", c.BaseURL, service)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return serviceops.Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return serviceops.Result{}, err
	}
	defer resp.Body.Close()

	var result serviceops.Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return serviceops.Result{}, err
	}
	return result, nil
}
