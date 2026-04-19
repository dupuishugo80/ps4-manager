package rpi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func decodeJSON(t *testing.T, body io.Reader, out any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(out); err != nil {
		t.Fatalf("decode request: %v", err)
	}
}

func TestNewClientRejectsInvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"no scheme", "192.168.1.1:12800"},
		{"no host", "http://"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.url); err == nil {
				t.Fatalf("expected error for %q", tc.url)
			}
		})
	}
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	client, err := NewClient("http://host:12800/")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.baseURL != "http://host:12800" {
		t.Fatalf("baseURL = %q", client.baseURL)
	}
}

func TestClientInstallDirect(t *testing.T) {
	var receivedPath string
	var received InstallRequest
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		decodeJSON(t, r.Body, &received)
		_, _ = w.Write([]byte(`{"status":"success","task_id":42,"title":"Bloodborne"}`))
	}))
	resp, err := client.Install(t.Context(), InstallRequest{
		Type:     PackageTypeDirect,
		Packages: []string{"http://pc/game.pkg"},
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if receivedPath != "/api/install" {
		t.Fatalf("path = %q", receivedPath)
	}
	if received.Type != PackageTypeDirect || len(received.Packages) != 1 {
		t.Fatalf("request = %+v", received)
	}
	if resp.TaskID != 42 || resp.Title != "Bloodborne" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestClientInstallRefPkgURL(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req InstallRequest
		decodeJSON(t, r.Body, &req)
		if req.Type != PackageTypeRefPkgURL || req.URL == "" {
			t.Errorf("unexpected body %+v", req)
		}
		_, _ = w.Write([]byte(`{"status":"success","task_id":7,"title":"Manifest"}`))
	}))
	resp, err := client.Install(t.Context(), InstallRequest{
		Type: PackageTypeRefPkgURL,
		URL:  "http://cdn/manifest.json",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if resp.TaskID != 7 {
		t.Fatalf("TaskID = %d", resp.TaskID)
	}
}

func TestClientInstallValidation(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("server should not be called")
	}))
	tests := []struct {
		name string
		req  InstallRequest
	}{
		{"direct without packages", InstallRequest{Type: PackageTypeDirect}},
		{"direct with url", InstallRequest{Type: PackageTypeDirect, Packages: []string{"x"}, URL: "y"}},
		{"ref without url", InstallRequest{Type: PackageTypeRefPkgURL}},
		{"ref with packages", InstallRequest{Type: PackageTypeRefPkgURL, URL: "y", Packages: []string{"x"}}},
		{"unknown type", InstallRequest{Type: "bogus"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.Install(t.Context(), tc.req); err == nil {
				t.Fatalf("expected validation error for %+v", tc.req)
			}
		})
	}
}

func TestClientIsExists(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     IsExistsResponse
		found    bool
	}{
		{
			name:     "found with size",
			response: `{"status":"success","exists":"true","size":0xFD4C65000}`,
			want:     IsExistsResponse{Exists: "true", Size: 67994275840},
			found:    true,
		},
		{
			name:     "not found no size",
			response: `{"status":"success","exists":"false"}`,
			want:     IsExistsResponse{Exists: "false"},
			found:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]string
				decodeJSON(t, r.Body, &req)
				if req["title_id"] != "CUSA12345" {
					t.Errorf("title_id = %q", req["title_id"])
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			resp, err := client.IsExists(t.Context(), "CUSA12345")
			if err != nil {
				t.Fatalf("IsExists: %v", err)
			}
			if resp.Exists != tc.want.Exists || resp.Size != tc.want.Size {
				t.Fatalf("resp = %+v, want %+v", resp, tc.want)
			}
			if resp.Found() != tc.found {
				t.Fatalf("Found() = %v, want %v", resp.Found(), tc.found)
			}
		})
	}
}

func TestClientUninstalls(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		idKey   string
		idValue string
		call    func(*Client, context.Context) error
	}{
		{
			name:    "uninstall game",
			path:    "/api/uninstall_game",
			idKey:   "title_id",
			idValue: "CUSA00001",
			call:    func(c *Client, ctx context.Context) error { return c.UninstallGame(ctx, "CUSA00001") },
		},
		{
			name:    "uninstall patch",
			path:    "/api/uninstall_patch",
			idKey:   "title_id",
			idValue: "CUSA00002",
			call:    func(c *Client, ctx context.Context) error { return c.UninstallPatch(ctx, "CUSA00002") },
		},
		{
			name:    "uninstall ac",
			path:    "/api/uninstall_ac",
			idKey:   "content_id",
			idValue: "UP0001-CUSA09311_00-ULCPACK000000004",
			call: func(c *Client, ctx context.Context) error {
				return c.UninstallAC(ctx, "UP0001-CUSA09311_00-ULCPACK000000004")
			},
		},
		{
			name:    "uninstall theme",
			path:    "/api/uninstall_theme",
			idKey:   "content_id",
			idValue: "UP9000-CUSA08344_00-DETROITCHARTHEME",
			call: func(c *Client, ctx context.Context) error {
				return c.UninstallTheme(ctx, "UP9000-CUSA08344_00-DETROITCHARTHEME")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Errorf("path = %q, want %q", r.URL.Path, tc.path)
				}
				var req map[string]string
				decodeJSON(t, r.Body, &req)
				if req[tc.idKey] != tc.idValue {
					t.Errorf("%s = %q, want %q", tc.idKey, req[tc.idKey], tc.idValue)
				}
				_, _ = w.Write([]byte(`{"status":"success"}`))
			}))
			if err := tc.call(client, t.Context()); err != nil {
				t.Fatalf("call: %v", err)
			}
		})
	}
}

func TestClientTaskLifecycle(t *testing.T) {
	tests := []struct {
		name string
		path string
		call func(*Client, context.Context) error
	}{
		{"start", "/api/start_task", func(c *Client, ctx context.Context) error { return c.StartTask(ctx, 99) }},
		{"stop", "/api/stop_task", func(c *Client, ctx context.Context) error { return c.StopTask(ctx, 99) }},
		{"pause", "/api/pause_task", func(c *Client, ctx context.Context) error { return c.PauseTask(ctx, 99) }},
		{"resume", "/api/resume_task", func(c *Client, ctx context.Context) error { return c.ResumeTask(ctx, 99) }},
		{"unregister", "/api/unregister_task", func(c *Client, ctx context.Context) error { return c.UnregisterTask(ctx, 99) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Errorf("path = %q", r.URL.Path)
				}
				var req map[string]int64
				decodeJSON(t, r.Body, &req)
				if req["task_id"] != 99 {
					t.Errorf("task_id = %d", req["task_id"])
				}
				_, _ = w.Write([]byte(`{"status":"success"}`))
			}))
			if err := tc.call(client, t.Context()); err != nil {
				t.Fatalf("call: %v", err)
			}
		})
	}
}

func TestClientGetTaskProgress(t *testing.T) {
	const raw = `{ "status": "success", "bits": 0xAB, "error": 0, "length": 0xFD4C65000, "transferred": 0x100, "length_total": 0xFD4C65000, "transferred_total": 0x100, "num_index": 1, "num_total": 3, "rest_sec": 120, "rest_sec_total": 300, "preparing_percent": 42, "local_copy_percent": -1 }`
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(raw))
	}))
	progress, err := client.GetTaskProgress(t.Context(), 123)
	if err != nil {
		t.Fatalf("GetTaskProgress: %v", err)
	}
	want := TaskProgress{
		Bits: 0xAB, Error: 0,
		Length: 0xFD4C65000, Transferred: 0x100,
		LengthTotal: 0xFD4C65000, TransferredTotal: 0x100,
		NumIndex: 1, NumTotal: 3,
		RestSec: 120, RestSecTotal: 300,
		PreparingPercent: 42, LocalCopyPercent: -1,
	}
	if *progress != want {
		t.Fatalf("progress = %+v, want %+v", *progress, want)
	}
}

func TestClientFindTask(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		decodeJSON(t, r.Body, &req)
		if req["content_id"] != "UP1004-CUSA03041_00-REDEMPTION000002" {
			t.Errorf("content_id = %v", req["content_id"])
		}
		if req["sub_type"].(float64) != float64(SubTypeGame) {
			t.Errorf("sub_type = %v", req["sub_type"])
		}
		_, _ = w.Write([]byte(`{"status":"success","task_id":55}`))
	}))
	resp, err := client.FindTask(t.Context(), "UP1004-CUSA03041_00-REDEMPTION000002", SubTypeGame)
	if err != nil {
		t.Fatalf("FindTask: %v", err)
	}
	if resp.TaskID != 55 {
		t.Fatalf("TaskID = %d", resp.TaskID)
	}
}

func TestClientAPIError(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     APIError
	}{
		{
			name:     "string error",
			response: `{"status":"fail","error":"Unable to set up prerequisites."}`,
			want:     APIError{Message: "Unable to set up prerequisites."},
		},
		{
			name:     "error code",
			response: `{"status":"fail","error_code":0x80990018}`,
			want:     APIError{ErrorCode: 0x80990018},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.response))
			}))
			_, err := client.Install(t.Context(), InstallRequest{Type: PackageTypeDirect, Packages: []string{"a"}})
			if err == nil {
				t.Fatalf("expected error")
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError, got %T: %v", err, err)
			}
			if apiErr.Message != tc.want.Message || apiErr.ErrorCode != tc.want.ErrorCode {
				t.Fatalf("apiErr = %+v, want %+v", apiErr, tc.want)
			}
		})
	}
}

func TestClientHTTPError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	err := client.StartTask(t.Context(), 1)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should mention status code, got %v", err)
	}
}

func TestClientInvalidJSON(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	if err := client.StartTask(t.Context(), 1); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestClientContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	err := client.StartTask(ctx, 1)
	if err == nil {
		t.Fatalf("expected context error")
	}
}
