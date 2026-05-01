package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/wnzhone/onespace/internal/domain"
)

type Result struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Checker struct {
	Client *http.Client
}

func (c Checker) Check(ctx context.Context, hc domain.HealthCheck) Result {
	if hc.Type == "" {
		return Result{Status: "unknown", Message: "no health check configured"}
	}

	if c.Client == nil {
		c.Client = http.DefaultClient
	}

	if hc.Type == "http" {
		if hc.URL == "" {
			return Result{Status: "unknown", Message: "no URL configured"}
		}

		reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout(hc.TimeoutSeconds))
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, hc.URL, nil)
		if err != nil {
			return Result{Status: "failing", Message: err.Error()}
		}

		resp, err := c.Client.Do(req)
		if err != nil {
			return Result{Status: "failing", Message: err.Error()}
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return Result{Status: "passing"}
		}
		return Result{Status: "failing", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	return Result{Status: "unknown", Message: fmt.Sprintf("unsupported health check type: %s", hc.Type)}
}

func defaultTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(seconds) * time.Second
}
