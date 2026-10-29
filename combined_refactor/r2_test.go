package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func withR2TestHooks(t *testing.T, client *http.Client) *[]string {
	t.Helper()
	var logs []string
	oldDelay := r2RetryDelay
	oldFactory := r2ClientFactory
	oldLogger := r2Logger
	r2RetryDelay = 0
	r2ClientFactory = func() *http.Client { return client }
	r2Logger = func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}
	t.Cleanup(func() {
		r2RetryDelay = oldDelay
		r2ClientFactory = oldFactory
		r2Logger = oldLogger
	})
	return &logs
}

func TestR2UploadSuccess(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/storage" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("token") != "secret-token" {
			t.Errorf("token query missing")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		if payload["filename"] != "targets.txt" || payload["value"] != "1.2.3.4:443#LAX\n" {
			t.Errorf("payload = %#v", payload)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logs := withR2TestHooks(t, server.Client())
	if err := uploadR2Content(server.URL, "secret-token", "targets.txt", "1.2.3.4:443#LAX\n"); err != nil {
		t.Fatalf("uploadR2Content: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d", got)
	}
	if len(*logs) == 0 || !strings.Contains(strings.Join(*logs, "\n"), "上传成功") {
		t.Fatalf("missing success log: %#v", *logs)
	}
}

func TestR2UploadRetriesThenSucceeds(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	if err := uploadR2Content(server.URL, "token", "", "content"); err != nil {
		t.Fatalf("uploadR2Content: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d", got)
	}
}

func TestR2UploadFailsAfterThreeAttempts(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	err := uploadR2Content(server.URL, "token", "targets.txt", "content")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d", got)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("error = %v", err)
	}
}

func TestR2UploadAuthFailureNoRetry(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				http.Error(w, "denied", status)
			}))
			defer server.Close()

			withR2TestHooks(t, server.Client())
			err := uploadR2Content(server.URL, "token", "targets.txt", "content")
			if err == nil {
				t.Fatal("expected error")
			}
			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Fatalf("attempts = %d", got)
			}
			if !strings.Contains(err.Error(), "鉴权") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestR2UploadSkipsEmptyContent(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
	}))
	defer server.Close()

	logs := withR2TestHooks(t, server.Client())
	if err := uploadR2Content(server.URL, "token", "targets.txt", " \n\t "); err != nil {
		t.Fatalf("uploadR2Content: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 0 {
		t.Fatalf("attempts = %d", got)
	}
	if !strings.Contains(strings.Join(*logs, "\n"), "跳过") {
		t.Fatalf("missing skip log: %#v", *logs)
	}
}

func TestR2UploadRejectsInvalidConfig(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		token  string
		substr string
	}{
		{"empty url", "", "token", "worker-url"},
		{"empty token", "https://example.com", " ", "worker-token"},
		{"bad scheme", "ftp://example.com", "token", "http/https"},
		{"no host", "https://", "token", "http/https"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withR2TestHooks(t, &http.Client{Timeout: time.Second})
			err := uploadR2Content(tc.url, tc.token, "targets.txt", "content")
			if err == nil || !strings.Contains(err.Error(), tc.substr) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestWriteExportUploadsSubstoreRegardlessOfLocalFormat(t *testing.T) {
	var stored map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		stored = nil
		if err := json.Unmarshal(body, &stored); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	t.Chdir(t.TempDir())
	rows := []cliResultRow{
		{"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛杉矶"},
		{"ip": "2606:4700::1", "port": "443", "dc": "HKG", "city": "香港"},
	}
	cfg := &cliConfig{
		outFile:       "export.csv",
		export:        cliExportConfig{Format: "csv"},
		r2Enabled:     true,
		r2WorkerURL:   server.URL,
		r2WorkerToken: "token",
		r2Filename:    "targets.txt",
	}
	if err := writeCLIExportAndMaybeUpload(cfg, rows, rows, "test"); err != nil {
		t.Fatalf("writeCLIExportAndMaybeUpload: %v", err)
	}
	want := "1.2.3.4:443#LAX-洛杉矶\n[2606:4700::1]:443#HKG-香港\n"
	if stored["filename"] != "targets.txt" || stored["value"] != want {
		t.Fatalf("uploaded payload = %#v", stored)
	}
	if _, err := os.Stat("export.csv"); err != nil {
		t.Fatalf("local export missing: %v", err)
	}
}

func TestWriteExportR2FailureKeepsOldContent(t *testing.T) {
	var failing atomic.Bool
	var stored string
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		if failing.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad payload: %v", err)
			return
		}
		stored = payload["value"]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	if err := uploadR2Content(server.URL, "token", "targets.txt", "old-content\n"); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	failing.Store(true)
	atomic.StoreInt32(&attempts, 0)

	t.Chdir(t.TempDir())
	rows := []cliResultRow{{"ip": "1.2.3.4", "port": "443", "dc": "LAX"}}
	cfg := &cliConfig{
		outFile:       "export.txt",
		export:        cliExportConfig{Format: "substore"},
		r2Enabled:     true,
		r2WorkerURL:   server.URL,
		r2WorkerToken: "token",
	}
	if err := writeCLIExportAndMaybeUpload(cfg, rows, rows, "test"); err != nil {
		t.Fatalf("upload failure must not fail export: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d", got)
	}
	if stored != "old-content\n" {
		t.Fatalf("stored content changed: %q", stored)
	}
	local, err := os.ReadFile("export.txt")
	if err != nil || !strings.Contains(string(local), "1.2.3.4:443#LAX") {
		t.Fatalf("local export missing substore content: %q, err=%v", local, err)
	}
}

func writeScheduledTestConfig(t *testing.T, dir, workerURL string) string {
	t.Helper()
	path := filepath.Join(dir, "cfdata-config.json")
	root := map[string]any{
		"config": map[string]any{
			"mode":          "official",
			"outformat":     "csv",
			"outstartrow":   1,
			"outendrow":     20,
			"r2enabled":     true,
			"r2workerurl":   workerURL,
			"r2workertoken": "secret",
			"r2filename":    "targets.txt",
		},
	}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadScheduledCLIConfigForcesSubstoreExport(t *testing.T) {
	path := writeScheduledTestConfig(t, t.TempDir(), "https://r2.example.com")
	cfg, err := loadScheduledCLIConfig(path)
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	if cfg.export.Format != "substore" {
		t.Fatalf("scheduled format = %q, want substore", cfg.export.Format)
	}
	if cfg.export.GHPath != "results/ip.txt" {
		t.Fatalf("scheduled ghpath = %q", cfg.export.GHPath)
	}
	if !cfg.r2Enabled || cfg.r2WorkerURL != "https://r2.example.com" || cfg.r2WorkerToken != "secret" || cfg.r2Filename != "targets.txt" {
		t.Fatalf("r2 config not carried: %#v", cfg)
	}
}

func TestScheduledExportEmptyResultsSkipsR2(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	t.Chdir(t.TempDir())
	cfg, err := loadScheduledCLIConfig(writeScheduledTestConfig(t, t.TempDir(), server.URL))
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	if err := writeCLIExportAndMaybeUpload(cfg, nil, nil, "schedule"); err != nil {
		t.Fatalf("writeCLIExportAndMaybeUpload: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 0 {
		t.Fatalf("empty results uploaded: attempts = %d", got)
	}
	if _, err := os.Stat("cfdata-results.txt"); !os.IsNotExist(err) {
		t.Fatalf("empty results should not create export, stat err = %v", err)
	}
}

func TestScheduledExportUploadsSubstoreContent(t *testing.T) {
	var stored map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		stored = nil
		if err := json.Unmarshal(body, &stored); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	t.Chdir(t.TempDir())
	cfg, err := loadScheduledCLIConfig(writeScheduledTestConfig(t, t.TempDir(), server.URL))
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	rows := []cliResultRow{
		{"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛杉矶"},
		{"ip": "2606:4700::1", "port": "443", "dc": "HKG", "city": "香港"},
	}
	if err := writeCLIExportAndMaybeUpload(cfg, rows, rows, "schedule"); err != nil {
		t.Fatalf("writeCLIExportAndMaybeUpload: %v", err)
	}
	want := "1.2.3.4:443#LAX-洛杉矶\n[2606:4700::1]:443#HKG-香港\n"
	if stored["filename"] != "targets.txt" || stored["value"] != want {
		t.Fatalf("uploaded payload = %#v", stored)
	}
	local, err := os.ReadFile("cfdata-results.txt")
	if err != nil {
		t.Fatalf("scheduled export missing: %v", err)
	}
	if !strings.Contains(string(local), want) {
		t.Fatalf("scheduled export not substore: %q", local)
	}
}

func TestSaveScheduleConfigR2RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	root := map[string]any{
		"config": map[string]any{
			"mode":           "nsb",
			"r2enabled":      false,
			"custom-unknown": "keep",
		},
		"_config_help": []string{"help"},
	}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	enabled := true
	workerURL := " https://r2.example.com "
	token := "tok-123"
	filename := " targets.txt "
	update := scheduleConfigUpdate{
		R2Enabled:     &enabled,
		R2WorkerURL:   &workerURL,
		R2WorkerToken: &token,
		R2Filename:    &filename,
	}
	if err := validateScheduleConfigUpdate(&update); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := saveScheduleConfigFields(path, update); err != nil {
		t.Fatalf("save: %v", err)
	}

	cfg, err := readScheduleConfigFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !cfg.R2Enabled || cfg.R2WorkerURL != "https://r2.example.com" || cfg.R2WorkerToken != token || cfg.R2Filename != "targets.txt" {
		t.Fatalf("readback = %#v", cfg)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := saved["_config_help"]; !ok {
		t.Fatalf("_config_help dropped: %s", raw)
	}
	inner, ok := saved["config"].(map[string]any)
	if !ok || inner["custom-unknown"] != "keep" {
		t.Fatalf("unknown config field dropped: %s", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
}

func TestConfigDataPayloadStripsR2Token(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	raw := []byte(`{"config":{"r2enabled":true,"r2workerurl":"https://r2.example.com","r2workertoken":"nested-secret-token"},"r2workertoken":"top-secret-token"}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	payload, err := buildConfigDataPayload(path)
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal sanitized config: %v", err)
	}
	if strings.Contains(string(data), "nested-secret-token") || strings.Contains(string(data), "top-secret-token") {
		t.Fatalf("config_data must not expose r2 workertoken: %s", data)
	}
	if !strings.Contains(string(data), "https://r2.example.com") {
		t.Fatalf("unrelated config fields must be preserved: %s", data)
	}
	if !strings.Contains(string(data), `"r2enabled":true`) {
		t.Fatalf("r2enabled flag must be preserved: %s", data)
	}
}

func TestScheduleConfigPayloadMasksR2Token(t *testing.T) {
	path := writeScheduledTestConfig(t, t.TempDir(), "https://r2.example.com")
	payload, err := scheduleConfigPayload(path)
	if err != nil {
		t.Fatalf("scheduleConfigPayload: %v", err)
	}
	if payload["r2WorkerToken"] != "" {
		t.Fatalf("payload must not expose stored token: %#v", payload["r2WorkerToken"])
	}
	if payload["r2WorkerTokenSet"] != true {
		t.Fatalf("payload should report stored token presence: %#v", payload["r2WorkerTokenSet"])
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatalf("serialized payload leaks token: %s", data)
	}
}

func TestValidateScheduleConfigUpdateR2Fields(t *testing.T) {
	invalid := []string{"ftp://example.com", "example.com", "https://", "://bad"}
	for _, raw := range invalid {
		value := raw
		update := scheduleConfigUpdate{R2WorkerURL: &value}
		if err := validateScheduleConfigUpdate(&update); err == nil {
			t.Fatalf("url %q should be rejected", raw)
		}
	}
	valid := " https://r2.example.com/base "
	update := scheduleConfigUpdate{R2WorkerURL: &valid}
	if err := validateScheduleConfigUpdate(&update); err != nil {
		t.Fatalf("valid url rejected: %v", err)
	}
	if *update.R2WorkerURL != "https://r2.example.com/base" {
		t.Fatalf("url not trimmed: %q", *update.R2WorkerURL)
	}
}

func TestScheduleConfigUpdateBlankTokenKeepsStored(t *testing.T) {
	path := writeScheduledTestConfig(t, t.TempDir(), "https://r2.example.com")
	blank := "   "
	update := scheduleConfigUpdate{R2WorkerToken: &blank}
	if err := validateScheduleConfigUpdate(&update); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if update.R2WorkerToken != nil {
		t.Fatalf("blank token should be dropped, got %q", *update.R2WorkerToken)
	}
	if err := saveScheduleConfigFields(path, update); err != nil {
		t.Fatalf("save: %v", err)
	}
	cfg, err := readScheduleConfigFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cfg.R2WorkerToken != "secret" {
		t.Fatalf("stored token overwritten: %q", cfg.R2WorkerToken)
	}
}

func TestR2UploadErrorDoesNotLeakToken(t *testing.T) {
	token := "super-secret-token"
	logs := withR2TestHooks(t, &http.Client{Timeout: 500 * time.Millisecond})
	err := uploadR2Content("http://127.0.0.1:1", token, "targets.txt", "content")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaks token: %v", err)
	}
	if strings.Contains(strings.Join(*logs, "\n"), token) {
		t.Fatalf("logs leak token: %#v", *logs)
	}
}

func TestR2ErrorRedactsTokenEchoedInResponseBody(t *testing.T) {
	const token = "echoed-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "upstream failed url=%s token=%s", r.URL.String(), token)
	}))
	defer server.Close()

	logs := withR2TestHooks(t, server.Client())
	err := uploadR2Content(server.URL, token, "targets.txt", "content")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaks token: %v", err)
	}
	if strings.Contains(strings.Join(*logs, "\n"), token) {
		t.Fatalf("logs leak token: %#v", *logs)
	}
	if !strings.Contains(err.Error(), "***") {
		t.Fatalf("error should contain redaction marker: %v", err)
	}
}

func TestExportAttemptsR2WhenGitHubUploadFails(t *testing.T) {
	var attempts int32
	var stored map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		body, _ := io.ReadAll(r.Body)
		stored = nil
		if err := json.Unmarshal(body, &stored); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	t.Chdir(t.TempDir())
	rows := []cliResultRow{{"ip": "1.2.3.4", "port": "443", "dc": "LAX"}}
	cfg := &cliConfig{
		outFile:       "export.txt",
		export:        cliExportConfig{Format: "substore", GitHub: true, GHRepo: "owner/repo"},
		r2Enabled:     true,
		r2WorkerURL:   server.URL,
		r2WorkerToken: "token",
		r2Filename:    "targets.txt",
	}
	err := writeCLIExportAndMaybeUpload(cfg, rows, rows, "test")
	if err == nil || !strings.Contains(err.Error(), "ghtoken") {
		t.Fatalf("expected GitHub token error, got %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("r2 attempts = %d, want 1 despite GitHub failure", got)
	}
	if stored["value"] != "1.2.3.4:443#LAX\n" {
		t.Fatalf("r2 payload = %#v", stored)
	}
}
