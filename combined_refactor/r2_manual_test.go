package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type manualR2LogCapture struct {
	mu    sync.Mutex
	lines []string
}

func (c *manualR2LogCapture) append(msgType string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, msgType+": "+fmt.Sprint(data))
}

func (c *manualR2LogCapture) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.lines, "\n")
}

func (c *manualR2LogCapture) waitFor(t *testing.T, sub string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if text := c.text(); strings.Contains(text, sub) {
			return text
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待日志 %q 超时，实际日志: %s", sub, c.text())
	return ""
}

func newManualR2Session(logs *manualR2LogCapture) *appSession {
	session := &appSession{}
	session.emit = logs.append
	return session
}

func writeManualR2Config(t *testing.T, fields map[string]any) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	data, err := json.Marshal(map[string]any{"config": fields})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	old := manualR2ConfigPath
	manualR2ConfigPath = func() string { return path }
	t.Cleanup(func() { manualR2ConfigPath = old })
}

func TestTriggerManualR2UploadWebSession(t *testing.T) {
	var calls int32
	var payload map[string]string
	done := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		payload = nil
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		done <- struct{}{}
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
	})
	session := &appSession{r2AutoUpload: true}
	rows := []cliResultRow{{"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛杉矶"}}

	triggerManualR2Upload(session, rows)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("手动任务未触发 R2 上传")
	}
	if payload["filename"] != "targets.txt" || payload["value"] != "1.2.3.4:443#LAX-洛杉矶\n" {
		t.Fatalf("上传内容 = %#v", payload)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("请求次数 = %d", got)
	}
}

func TestManualR2UploadSuccessLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
	})
	logs := &manualR2LogCapture{}
	session := newManualR2Session(logs)
	uploadManualRowsToR2(session, "1.2.3.4:443#LAX\n")
	logs.waitFor(t, "R2 上传完成：1 条结果已上传到 targets.txt")
}

func TestTriggerManualR2UploadSkipsCLISession(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
	})
	session := &appSession{r2AutoUpload: true, emit: func(string, interface{}) {}}
	triggerManualR2Upload(session, []cliResultRow{{"ip": "1.2.3.4", "port": "443"}})
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("CLI 会话不应触发手动上传，请求次数 = %d", got)
	}
}

func TestManualR2UploadDisabledSkips(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     false,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
	})
	logs := &manualR2LogCapture{}
	session := newManualR2Session(logs)
	uploadManualRowsToR2(session, "1.2.3.4:443#LAX\n")
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("未启用 R2 时不应发起请求，请求次数 = %d", got)
	}
	if !strings.Contains(logs.text(), "R2 上传未启用") {
		t.Fatalf("缺少未启用跳过提示: %s", logs.text())
	}
}

func TestManualR2UploadMissingConfigSkips(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]any
	}{
		{name: "missing_url", fields: map[string]any{"r2enabled": true, "r2workertoken": "secret"}},
		{name: "missing_token", fields: map[string]any{"r2enabled": true, "r2workerurl": "https://r2.example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
			}))
			defer server.Close()
			withR2TestHooks(t, server.Client())
			writeManualR2Config(t, tc.fields)
			logs := &manualR2LogCapture{}
			session := newManualR2Session(logs)
			uploadManualRowsToR2(session, "1.2.3.4:443#LAX\n")
			if got := atomic.LoadInt32(&calls); got != 0 {
				t.Fatalf("配置缺失时不应发起请求，请求次数 = %d", got)
			}
			if !strings.Contains(logs.text(), "Worker 地址或 token 未配置") {
				t.Fatalf("缺少跳过提示: %s", logs.text())
			}
		})
	}

	t.Run("missing_config_file", func(t *testing.T) {
		old := manualR2ConfigPath
		manualR2ConfigPath = func() string { return filepath.Join(t.TempDir(), "missing.json") }
		t.Cleanup(func() { manualR2ConfigPath = old })
		logs := &manualR2LogCapture{}
		session := newManualR2Session(logs)
		uploadManualRowsToR2(session, "1.2.3.4:443#LAX\n")
		if !strings.Contains(logs.text(), "读取 R2 配置失败") {
			t.Fatalf("缺少读取失败提示: %s", logs.text())
		}
	})
}

func TestManualR2UploadFailureKeepsOldContent(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
	})
	logs := &manualR2LogCapture{}
	session := newManualR2Session(logs)
	uploadManualRowsToR2(session, "1.2.3.4:443#LAX\n")
	if got := atomic.LoadInt32(&calls); got != r2UploadAttempts {
		t.Fatalf("失败重试次数 = %d", got)
	}
	if !strings.Contains(logs.text(), "R2 上传失败，保留 R2 上的旧内容") {
		t.Fatalf("缺少失败提示: %s", logs.text())
	}
}

func TestManualR2UploadErrorRedactsToken(t *testing.T) {
	const token = "super-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied "+r.URL.String(), http.StatusForbidden)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": token,
	})
	logs := &manualR2LogCapture{}
	session := newManualR2Session(logs)
	uploadManualRowsToR2(session, "1.2.3.4:443#LAX\n")
	text := logs.text()
	if !strings.Contains(text, "鉴权") {
		t.Fatalf("缺少鉴权失败提示: %s", text)
	}
	if strings.Contains(text, token) {
		t.Fatalf("日志泄露了 token: %s", text)
	}
}

func TestTriggerManualR2UploadEmptyRowsSkips(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
	})
	logs := &manualR2LogCapture{}
	session := newManualR2Session(logs)
	session.r2AutoUpload = true
	triggerManualR2Upload(session, nil)
	triggerManualR2Upload(session, []cliResultRow{{"ip": ""}})
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("空结果不应上传，请求次数 = %d", got)
	}
}

func TestOfficialResultRowsSubstoreContent(t *testing.T) {
	results := []TestResult{{
		IP:         "1.2.3.4",
		Port:       443,
		DataCenter: "LAX",
		City:       "洛杉矶",
		Speed:      "12.00MB/s",
		AvgLatency: 30 * time.Millisecond,
	}}
	content := formatSubstoreResults(officialResultRows(nil, results, scanModeTCPing))
	if content != "1.2.3.4:443#LAX-洛杉矶\n" {
		t.Fatalf("官方结果 Sub-Store 内容 = %q", content)
	}
}

func TestNSBSpeedResultRowsMergesUpdatedSpeed(t *testing.T) {
	rows := []nsbScanMessage{
		{IP: "1.2.3.4", Port: "443", DC: "LAX", City: "洛杉矶", Speed: "未测速"},
		{IP: "5.6.7.8", Port: "8443", DC: "HKG", City: "香港", Speed: "5.00MB/s"},
		{IP: "9.9.9.9", Port: "443", DC: "NRT", City: "东京", Speed: "7.00MB/s"},
	}
	results := []iptestResult{
		{
			ipAddr:         "1.2.3.4",
			port:           443,
			speedText:      "12.34MB/s",
			speedTested:    true,
			speedQualified: true,
		},
		{
			ipAddr: "9.9.9.9",
			port:   443,
		},
	}
	merged := nsbSpeedResultRows(rows, results)
	if len(merged) != 3 {
		t.Fatalf("合并后行数 = %d", len(merged))
	}
	if merged[0]["speed"] != "12.34MB/s" {
		t.Fatalf("本次测速行未更新: %#v", merged[0])
	}
	if merged[1]["speed"] != "5.00MB/s" {
		t.Fatalf("未测速行被覆盖: %#v", merged[1])
	}
	if merged[2]["speed"] != "7.00MB/s" {
		t.Fatalf("早停未测速行被清空: %#v", merged[2])
	}
	content := formatSubstoreResults(merged)
	if content != "1.2.3.4:443#LAX-洛杉矶\n5.6.7.8:8443#HKG-香港\n9.9.9.9:443#NRT-东京\n" {
		t.Fatalf("非标测速 Sub-Store 内容 = %q", content)
	}
}

func TestRunDetailedTestTriggersManualR2Upload(t *testing.T) {
	var payload map[string]string
	done := make(chan struct{}, 1)
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload = nil
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		done <- struct{}{}
	}))
	defer worker.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port

	withR2TestHooks(t, worker.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   worker.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
	})
	session := &appSession{r2AutoUpload: true}
	session.scanResults = []ScanResult{{
		IP:          "127.0.0.1",
		Port:        port,
		DataCenter:  "LAX",
		City:        "洛杉矶",
		LatencyStr:  "1ms",
		TCPDuration: time.Millisecond,
	}}
	runDetailedTest(context.Background(), session, "LAX", port, 0, scanModeTCPing, false)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("详细测试完成后未触发 R2 上传")
	}
	wantPrefix := fmt.Sprintf("127.0.0.1:%d#LAX", port)
	if !strings.HasPrefix(payload["value"], wantPrefix) {
		t.Fatalf("上传内容 = %q，期望前缀 %q", payload["value"], wantPrefix)
	}
}

func TestRunNSBTaskTriggersManualR2Upload(t *testing.T) {
	var payload map[string]string
	done := make(chan struct{}, 1)
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload = nil
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		done <- struct{}{}
	}))
	defer worker.Close()

	trace := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdn-cgi/trace" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, "colo=LAX\nloc=US\nip=1.2.3.4\nvisit_scheme=http\nhttp=http/1.1\n")
	}))
	defer trace.Close()
	_, tracePort, err := net.SplitHostPort(strings.TrimPrefix(trace.URL, "http://"))
	if err != nil {
		t.Fatalf("parse trace address: %v", err)
	}

	withR2TestHooks(t, worker.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   worker.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
	})
	t.Chdir(t.TempDir())
	session := &appSession{r2AutoUpload: true}
	input := "127.0.0.1 " + tracePort + "\n"
	runNSBTask(context.Background(), session, "local.txt", input, filepath.Join(t.TempDir(), "out.csv"), 1, 443, 0, "", false, 0, 10, "", 0.1, 0, false, scanModeTCPing, "")

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("非标任务完成后未触发 R2 上传")
	}
	if payload["filename"] != "targets.txt" {
		t.Fatalf("上传文件名 = %q", payload["filename"])
	}
	wantPrefix := "127.0.0.1:" + tracePort + "#LAX"
	if !strings.HasPrefix(payload["value"], wantPrefix) {
		t.Fatalf("上传内容 = %q，期望前缀 %q", payload["value"], wantPrefix)
	}
}

// 自动测速开启但测速上限为 0 时，批量测速不会启动，详细测试阶段必须自己上传，
// 否则该任务不会有任何 R2 上传（A2 的「未执行测速」场景）。
func TestOfficialAutoSpeedWithoutLimitKeepsDetailedUpload(t *testing.T) {
	data, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(data)
	want := "deferR2Upload: officialAutoSpeedPending && officialSpeedLimitForAuto() > 0"
	if !strings.Contains(html, want) {
		t.Fatalf("start_test 的 deferR2Upload 必须受测速上限约束，缺少: %q", want)
	}
	if strings.Contains(html, "deferR2Upload: officialAutoSpeedPending,") {
		t.Fatal("start_test 不应在测速上限为 0 时延后 R2 上传")
	}
	helper := "function officialSpeedLimitForAuto()"
	if !strings.Contains(html, helper) {
		t.Fatalf("缺少测速上限读取函数: %q", helper)
	}
}

func TestAutomationSectionExplainsGitHubAndR2Upload(t *testing.T) {
	data, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(data)
	for _, want := range []string{
		"自动调度扫描按规则上传：开启测速只上传测速达标行，未开启测速只上传通过延迟筛选的结果行",
		"手动任务需在结果页「⬆ 自动上传」下拉框选择「R2 上传」",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("自动化区块缺少说明文案: %q", want)
		}
	}
}

func TestTriggerManualR2UploadRequiresSelection(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
	})
	session := &appSession{}
	triggerManualR2Upload(session, []cliResultRow{{"ip": "1.2.3.4", "port": "443"}})
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("未选择 R2 时不应发起请求，请求次数 = %d", got)
	}
}

// 后台自动测速在详细测试完成之后、测速开始之前被取消时，测速阶段因 ctx 已取消不上传，
// 调用方必须补传一次延迟筛选结果，否则该窗口 R2 完全收不到内容。
func TestOfficialSpeedBatchCanceledBeforeStartSkipsUpload(t *testing.T) {
	var calls int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer worker.Close()

	withR2TestHooks(t, worker.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   worker.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
	})
	session := &appSession{r2AutoUpload: true}
	session.testResults = []TestResult{{IP: "1.2.3.4", Port: 443, DataCenter: "LAX", City: "洛杉矶", Speed: ""}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runOfficialSpeedBatch(ctx, session, 443, "http://example.invalid/speed", 5, 0.1, nil, false)
	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("测速开始前取消不应由测速阶段上传，请求次数 = %d", got)
	}
}

func TestOfficialSpeedAttemptedDetectsPartialResults(t *testing.T) {
	session := &appSession{}
	session.testResults = []TestResult{
		{IP: "1.2.3.4", Speed: ""},
		{IP: "5.6.7.8", Speed: "未测速"},
	}
	if officialSpeedAttempted(session) {
		t.Fatal("空速度与未测速不应视为已测速")
	}
	session.testResults[0].Speed = "1.20MB/s"
	if !officialSpeedAttempted(session) {
		t.Fatal("已有测速结果应视为已测速")
	}
	session.testResults[0].Speed = "测速失败"
	if !officialSpeedAttempted(session) {
		t.Fatal("测速失败也应视为已测速，取消后不再补传")
	}
}

func TestRunDetailedTestDeferR2UploadSkipsUpload(t *testing.T) {
	var calls int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer worker.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port

	withR2TestHooks(t, worker.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   worker.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
	})
	session := &appSession{r2AutoUpload: true}
	session.scanResults = []ScanResult{{
		IP:          "127.0.0.1",
		Port:        port,
		DataCenter:  "LAX",
		City:        "洛杉矶",
		LatencyStr:  "1ms",
		TCPDuration: time.Millisecond,
	}}
	runDetailedTest(context.Background(), session, "LAX", port, 0, scanModeTCPing, true)
	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("自动测速流程的详细测试阶段不应上传 R2，请求次数 = %d", got)
	}
}

func TestScheduledR2RowsFollowSpeedRule(t *testing.T) {
	rows := []cliResultRow{
		{"ip": "1.2.3.4", "port": "443", "dc": "LAX", "speed": "5.00MB/s"},
		{"ip": "5.6.7.8", "port": "443", "dc": "HKG", "speed": "0.05MB/s"},
		{"ip": "9.9.9.9", "port": "443", "dc": "NRT", "speed": ""},
	}
	if got := scheduledR2Rows(rows, false, 0.1); len(got) != 3 {
		t.Fatalf("未开启测速应保留全部行，实际 %d", len(got))
	}
	got := scheduledR2Rows(rows, true, 0.1)
	if len(got) != 1 || got[0]["ip"] != "1.2.3.4" {
		t.Fatalf("开启测速只应保留达标行，实际 %#v", got)
	}
}

func TestScheduledR2UploadIgnoresExportFilters(t *testing.T) {
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
	exportRows := []cliResultRow{{"ip": "1.2.3.4", "port": "443", "dc": "LAX"}}
	r2Rows := []cliResultRow{
		{"ip": "1.2.3.4", "port": "443", "dc": "LAX"},
		{"ip": "5.6.7.8", "port": "443", "dc": "HKG"},
	}
	cfg := &cliConfig{
		outFile:       "export.txt",
		export:        cliExportConfig{Format: "substore"},
		r2Enabled:     true,
		r2WorkerURL:   server.URL,
		r2WorkerToken: "token",
		r2Filename:    "targets.txt",
	}
	if err := writeCLIExportAndMaybeUpload(cfg, exportRows, r2Rows, "schedule"); err != nil {
		t.Fatalf("writeCLIExportAndMaybeUpload: %v", err)
	}
	if got, want := stored["value"], "1.2.3.4:443#LAX\n5.6.7.8:443#HKG\n"; got != want {
		t.Fatalf("R2 内容 = %q，期望 %q", got, want)
	}
	local, err := os.ReadFile("export.txt")
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if strings.Contains(string(local), "5.6.7.8") {
		t.Fatalf("导出文件不应包含被筛选掉的行: %q", local)
	}
}

func TestScheduledR2UploadsWithoutExportFile(t *testing.T) {
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
	cfg := &cliConfig{
		outFile:       "export.txt",
		export:        cliExportConfig{Format: "substore"},
		r2Enabled:     true,
		r2WorkerURL:   server.URL,
		r2WorkerToken: "token",
		r2Filename:    "targets.txt",
	}
	r2Rows := []cliResultRow{{"ip": "1.2.3.4", "port": "443", "dc": "LAX"}}
	if err := writeCLIExportAndMaybeUpload(cfg, nil, r2Rows, "schedule"); err != nil {
		t.Fatalf("writeCLIExportAndMaybeUpload: %v", err)
	}
	if got, want := stored["value"], "1.2.3.4:443#LAX\n"; got != want {
		t.Fatalf("R2 内容 = %q，期望 %q", got, want)
	}
	if _, err := os.Stat("export.txt"); !os.IsNotExist(err) {
		t.Fatalf("没有导出行时不应创建导出文件，stat err = %v", err)
	}
}
