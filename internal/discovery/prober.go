package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// Signature GoldHEN RPI returns for any non-POST request on /api/.
	rpiUnsupportedMarker = "Unsupported method"
	rpiStatusFail        = "fail"

	defaultProbeTimeout = 500 * time.Millisecond
)

type rpiErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type HTTPProber struct {
	client  *http.Client
	timeout time.Duration
}

type HTTPProberOption func(*HTTPProber)

func WithHTTPClient(client *http.Client) HTTPProberOption {
	return func(p *HTTPProber) { p.client = client }
}

func WithProbeTimeout(d time.Duration) HTTPProberOption {
	return func(p *HTTPProber) { p.timeout = d }
}

func NewHTTPProber(opts ...HTTPProberOption) *HTTPProber {
	prober := &HTTPProber{
		client:  &http.Client{Timeout: defaultProbeTimeout},
		timeout: defaultProbeTimeout,
	}
	for _, opt := range opts {
		opt(prober)
	}
	return prober
}

func (p *HTTPProber) Probe(parentCtx context.Context, host string, port int) error {
	probeCtx, cancel := context.WithTimeout(parentCtx, p.timeout)
	defer cancel()
	url := fmt.Sprintf("http://%s/api/", joinHostPort(host, port))
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build probe request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", url, err)
	}
	defer func() {
		// Skip: body already consumed, close error is not actionable.
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return fmt.Errorf("read probe body: %w", err)
	}
	var payload rpiErrorResponse
	if decodeErr := json.Unmarshal(body, &payload); decodeErr != nil {
		return fmt.Errorf("%w: invalid json", ErrNotRPI)
	}
	if payload.Status != rpiStatusFail || payload.Error != rpiUnsupportedMarker {
		return fmt.Errorf("%w: unexpected payload", ErrNotRPI)
	}
	return nil
}
