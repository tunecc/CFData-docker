package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// 结果操作面板「上传 R2」按钮的服务端处理：校验、上传与 token 保护。

func TestUploadPanelContentToR2EmptyContent(t *testing.T) {
	logs := &manualR2LogCapture{}
	uploadPanelContentToR2(newManualR2Session(logs), "   \n")
	text := logs.waitFor(t, "r2_upload_error")
	if !strings.Contains(text, "没有可上传的结果") {
		t.Fatalf("空内容应提示没有可上传结果，实际: %s", text)
	}
}

func TestUploadPanelContentToR2DisabledConfig(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writeManualR2Config(t, map[string]any{
		"r2enabled":     false,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
	})
	logs := &manualR2LogCapture{}
	uploadPanelContentToR2(newManualR2Session(logs), "1.2.3.4:443#LAX\n")
	text := logs.waitFor(t, "r2_upload_error")
	if !strings.Contains(text, "R2 上传未启用") {
		t.Fatalf("未启用时应提示开启，实际: %s", text)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("未启用时不应发起请求，请求次数 = %d", got)
	}
}

func TestUploadPanelContentToR2MissingWorkerConfig(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writeManualR2Config(t, map[string]any{
		"r2enabled":   true,
		"r2workerurl": server.URL,
	})
	logs := &manualR2LogCapture{}
	uploadPanelContentToR2(newManualR2Session(logs), "1.2.3.4:443#LAX\n")
	text := logs.waitFor(t, "r2_upload_error")
	if !strings.Contains(text, "Worker 地址或 token 未配置") {
		t.Fatalf("缺少 token 时应提示配置不完整，实际: %s", text)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("配置不完整时不应发起请求，请求次数 = %d", got)
	}
}

func TestUploadPanelContentToR2ConfigReadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	old := manualR2ConfigPath
	manualR2ConfigPath = func() string { return path }
	t.Cleanup(func() { manualR2ConfigPath = old })

	logs := &manualR2LogCapture{}
	uploadPanelContentToR2(newManualR2Session(logs), "1.2.3.4:443#LAX\n")
	if text := logs.text(); !strings.Contains(text, "读取 R2 配置失败") {
		t.Fatalf("配置读取失败应回报明确错误，实际: %s", text)
	}
}

func TestUploadPanelContentToR2UploadsPanelContent(t *testing.T) {
	var calls int32
	var payload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		payload = nil
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret-token",
		"r2filename":    "panel-targets.txt",
	})
	logs := withR2TestHooks(t, server.Client())
	logs2 := &manualR2LogCapture{}
	uploadPanelContentToR2(newManualR2Session(logs2), "1.2.3.4:443#LAX\n5.6.7.8:2053#HKG\n")

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("应发起一次上传请求，请求次数 = %d", got)
	}
	if payload["filename"] != "panel-targets.txt" {
		t.Fatalf("目标文件名应取已保存配置，实际: %q", payload["filename"])
	}
	if !strings.Contains(payload["value"], "1.2.3.4:443#LAX") || !strings.Contains(payload["value"], "5.6.7.8:2053#HKG") {
		t.Fatalf("上传内容应为面板生成内容原样提交，实际: %q", payload["value"])
	}
	text := logs2.waitFor(t, "r2_upload_result")
	if !strings.Contains(text, "panel-targets.txt") || !strings.Contains(text, "2 行") {
		t.Fatalf("成功回执应包含文件名与行数，实际: %s", text)
	}
	// token 不允许出现在 WS 回执与 r2 日志中
	if strings.Contains(text, "secret-token") {
		t.Fatalf("WS 回执不应包含 token 明文: %s", text)
	}
	for _, line := range *logs {
		if strings.Contains(line, "secret-token") {
			t.Fatalf("R2 日志不应包含 token 明文: %s", line)
		}
	}
}

func TestUploadPanelContentToR2DefaultFilename(t *testing.T) {
	var payload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload = nil
		_ = json.Unmarshal(body, &payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret-token",
	})
	withR2TestHooks(t, server.Client())
	logs := &manualR2LogCapture{}
	uploadPanelContentToR2(newManualR2Session(logs), "1.2.3.4:443#LAX\n")

	if payload["filename"] != defaultR2Filename {
		t.Fatalf("未配置文件名时应使用默认值 %s，实际: %q", defaultR2Filename, payload["filename"])
	}
}

func TestUploadPanelContentToR2FailureKeepsOldContent(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret-token",
		"r2filename":    "panel-targets.txt",
	})
	withR2TestHooks(t, server.Client())
	logs := &manualR2LogCapture{}
	uploadPanelContentToR2(newManualR2Session(logs), "1.2.3.4:443#LAX\n")

	text := logs.waitFor(t, "r2_upload_error")
	if !strings.Contains(text, "R2 上传失败") || !strings.Contains(text, "保留 R2 上的旧内容") {
		t.Fatalf("失败应回报原因并说明保留旧内容，实际: %s", text)
	}
	if strings.Contains(text, "secret-token") {
		t.Fatalf("失败回执不应包含 token 明文: %s", text)
	}
}
