package discovery

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func serverHost(t *testing.T, server *httptest.Server) (string, int) {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	host, portStr, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return host, port
}

func TestHTTPProberAcceptsRPISignature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{ "status": "fail", "error": "Unsupported method" }`))
	}))
	defer server.Close()
	prober := NewHTTPProber(WithProbeTimeout(2 * time.Second))
	host, port := serverHost(t, server)
	if err := prober.Probe(context.Background(), host, port); err != nil {
		t.Fatalf("Probe: %v", err)
	}
}

func TestHTTPProberRejects(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "non json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("<html>hello</html>"))
			},
		},
		{
			name: "json but wrong shape",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			},
		},
		{
			name: "json status ok",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"ok","error":"Unsupported method"}`))
			},
		},
		{
			name: "json wrong error field",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"fail","error":"Something else"}`))
			},
		},
		{
			name: "empty body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
		},
	}
	prober := NewHTTPProber(WithProbeTimeout(2 * time.Second))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			host, port := serverHost(t, server)
			err := prober.Probe(context.Background(), host, port)
			if !errors.Is(err, ErrNotRPI) {
				t.Fatalf("expected ErrNotRPI, got %v", err)
			}
		})
	}
}

func TestHTTPProberTimesOut(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(block)
		server.Close()
	})
	prober := NewHTTPProber(WithProbeTimeout(50 * time.Millisecond))
	host, port := serverHost(t, server)
	err := prober.Probe(context.Background(), host, port)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if errors.Is(err, ErrNotRPI) {
		t.Fatalf("timeout should not be reported as ErrNotRPI: %v", err)
	}
}

func TestHTTPProberParentContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"fail","error":"Unsupported method"}`))
	}))
	defer server.Close()
	prober := NewHTTPProber(WithProbeTimeout(2 * time.Second))
	host, port := serverHost(t, server)
	parentCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err := prober.Probe(parentCtx, host, port)
	if err == nil {
		t.Fatalf("expected error on cancelled context")
	}
	if !strings.Contains(err.Error(), "probe") {
		t.Fatalf("error should mention probe context, got %v", err)
	}
}

func TestHTTPProberConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	_ = listener.Close()
	prober := NewHTTPProber(WithProbeTimeout(500 * time.Millisecond))
	probeErr := prober.Probe(context.Background(), "127.0.0.1", addr.Port)
	if probeErr == nil {
		t.Fatalf("expected connection error")
	}
	if errors.Is(probeErr, ErrNotRPI) {
		t.Fatalf("network error should not be ErrNotRPI: %v", probeErr)
	}
}

func TestHTTPProberUsesInjectedClient(t *testing.T) {
	called := false
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       httpBody(`{"status":"fail","error":"Unsupported method"}`),
			Header:     make(http.Header),
			Request:    r,
		}
		return resp, nil
	})
	prober := NewHTTPProber(WithHTTPClient(&http.Client{Transport: rt}))
	if err := prober.Probe(context.Background(), "1.2.3.4", 12800); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !called {
		t.Fatalf("injected transport was not used")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func httpBody(s string) readCloser {
	return readCloser{strings.NewReader(s)}
}

type readCloser struct{ *strings.Reader }

func (readCloser) Close() error { return nil }
