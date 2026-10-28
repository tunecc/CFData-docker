package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type officialTestMessage struct {
	msgType string
	data    interface{}
}

type officialTestCapture struct {
	mu       sync.Mutex
	messages []officialTestMessage
}

func (c *officialTestCapture) append(msgType string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, officialTestMessage{msgType: msgType, data: data})
}

func (c *officialTestCapture) ofType(msgType string) []interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	var result []interface{}
	for _, message := range c.messages {
		if message.msgType == msgType {
			result = append(result, message.data)
		}
	}
	return result
}

func (c *officialTestCapture) contains(msgType, sub string) bool {
	for _, data := range c.ofType(msgType) {
		if strings.Contains(fmt.Sprint(data), sub) {
			return true
		}
	}
	return false
}

func newOfficialTestSession() (*appSession, *officialTestCapture) {
	capture := &officialTestCapture{}
	session := &appSession{}
	session.emit = capture.append
	return session, capture
}

func officialCandidateIPs(count int) []string {
	ips := make([]string, count)
	for i := range ips {
		ips[i] = fmt.Sprintf("10.0.0.%d", i+1)
	}
	return ips
}

func officialStubScanner(dataCenter string, calls *int64) officialScanFunc {
	return func(ctx context.Context, ip string, port int, delay int) (*ScanResult, string, string) {
		atomic.AddInt64(calls, 1)
		return &ScanResult{IP: ip, Port: port, DataCenter: dataCenter, LatencyStr: "1ms", TCPDuration: time.Millisecond}, "", ""
	}
}

func officialMixedScanner(dcByIP map[string]string, calls *int64) officialScanFunc {
	return func(ctx context.Context, ip string, port int, delay int) (*ScanResult, string, string) {
		atomic.AddInt64(calls, 1)
		return &ScanResult{IP: ip, Port: port, DataCenter: dcByIP[ip], LatencyStr: "1ms", TCPDuration: time.Millisecond}, "", ""
	}
}

func officialSessionResults(session *appSession) []ScanResult {
	session.scanMutex.Lock()
	defer session.scanMutex.Unlock()
	return append([]ScanResult(nil), session.scanResults...)
}

func officialDCListFromMessages(t *testing.T, messages []interface{}) []DataCenterInfo {
	t.Helper()
	if len(messages) != 1 {
		t.Fatalf("scan_complete_wait_dc 次数 = %d, want 1", len(messages))
	}
	dcList, ok := messages[0].([]DataCenterInfo)
	if !ok {
		t.Fatalf("scan_complete_wait_dc 类型 = %T", messages[0])
	}
	return dcList
}

func TestOfficialScanFiltersTargetDCOnly(t *testing.T) {
	ips := officialCandidateIPs(10)
	dcByIP := make(map[string]string, len(ips))
	for i, ip := range ips {
		if i%2 == 0 {
			dcByIP[ip] = "HKG"
		} else {
			dcByIP[ip] = "LAX"
		}
	}
	var calls int64
	session, capture := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(context.Background(), session, ips, 4, 443, 500, "hkg", 0, officialMixedScanner(dcByIP, &calls))
	if wasCanceled || stoppedEarly {
		t.Fatalf("wasCanceled=%v stoppedEarly=%v, want false/false", wasCanceled, stoppedEarly)
	}
	results := officialSessionResults(session)
	if len(results) != 5 {
		t.Fatalf("结果数量 = %d, want 5", len(results))
	}
	for _, res := range results {
		if res.DataCenter != "HKG" {
			t.Fatalf("非目标数据中心结果被保留: %#v", res)
		}
	}
	if got := atomic.LoadInt64(&calls); got != 10 {
		t.Fatalf("扫描次数 = %d, want 10", got)
	}
	dcList := officialDCListFromMessages(t, capture.ofType("scan_complete_wait_dc"))
	if len(dcList) != 1 || dcList[0].DataCenter != "HKG" || dcList[0].IPCount != 5 {
		t.Fatalf("数据中心汇总 = %#v", dcList)
	}
}

func TestOfficialScanEarlyStopsAtLimit(t *testing.T) {
	ips := officialCandidateIPs(100)
	var calls int64
	session, capture := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(context.Background(), session, ips, 1, 443, 500, "HKG", 3, officialStubScanner("HKG", &calls))
	if wasCanceled || !stoppedEarly {
		t.Fatalf("wasCanceled=%v stoppedEarly=%v, want false/true", wasCanceled, stoppedEarly)
	}
	results := officialSessionResults(session)
	if len(results) != 3 {
		t.Fatalf("结果数量 = %d, want 3", len(results))
	}
	if got := atomic.LoadInt64(&calls); got != 3 {
		t.Fatalf("扫描次数 = %d, want 3", got)
	}
	if !capture.contains("log", "提前结束扫描") {
		t.Fatalf("缺少提前结束日志: %#v", capture.ofType("log"))
	}
	officialDCListFromMessages(t, capture.ofType("scan_complete_wait_dc"))
}

func TestOfficialScanEarlyStopAllowsInFlight(t *testing.T) {
	ips := officialCandidateIPs(50)
	var calls int64
	release := make(chan struct{})
	var releaseOnce sync.Once
	scanner := officialScanFunc(func(ctx context.Context, ip string, port int, delay int) (*ScanResult, string, string) {
		n := atomic.AddInt64(&calls, 1)
		if n >= 3 {
			releaseOnce.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
			return nil, "test_timeout", "release timeout"
		}
		return &ScanResult{IP: ip, Port: port, DataCenter: "HKG", LatencyStr: "1ms", TCPDuration: time.Millisecond}, "", ""
	})
	session, _ := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(context.Background(), session, ips, 3, 443, 500, "HKG", 1, scanner)
	if wasCanceled || !stoppedEarly {
		t.Fatalf("wasCanceled=%v stoppedEarly=%v, want false/true", wasCanceled, stoppedEarly)
	}
	if results := officialSessionResults(session); len(results) != 3 {
		t.Fatalf("在途请求结果数量 = %d, want 3", len(results))
	}
	if got := atomic.LoadInt64(&calls); got != 3 {
		t.Fatalf("在途扫描次数 = %d, want 3", got)
	}
}

func TestOfficialScanWithoutTargetDCScansAll(t *testing.T) {
	ips := officialCandidateIPs(10)
	dcByIP := make(map[string]string, len(ips))
	for i, ip := range ips {
		if i%2 == 0 {
			dcByIP[ip] = "HKG"
		} else {
			dcByIP[ip] = "LAX"
		}
	}
	var calls int64
	session, capture := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(context.Background(), session, ips, 4, 443, 500, "", 20, officialMixedScanner(dcByIP, &calls))
	if wasCanceled || stoppedEarly {
		t.Fatalf("无目标数据中心不应早停: canceled=%v early=%v", wasCanceled, stoppedEarly)
	}
	if results := officialSessionResults(session); len(results) != 10 {
		t.Fatalf("结果数量 = %d, want 10", len(results))
	}
	if got := atomic.LoadInt64(&calls); got != 10 {
		t.Fatalf("扫描次数 = %d, want 10", got)
	}
	dcList := officialDCListFromMessages(t, capture.ofType("scan_complete_wait_dc"))
	if len(dcList) != 2 {
		t.Fatalf("数据中心汇总数量 = %d, want 2", len(dcList))
	}
}

func TestOfficialScanTargetDCWithStopZeroScansAll(t *testing.T) {
	ips := officialCandidateIPs(10)
	dcByIP := make(map[string]string, len(ips))
	for i, ip := range ips {
		if i%2 == 0 {
			dcByIP[ip] = "HKG"
		} else {
			dcByIP[ip] = "LAX"
		}
	}
	var calls int64
	session, _ := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(context.Background(), session, ips, 4, 443, 500, "HKG", 0, officialMixedScanner(dcByIP, &calls))
	if wasCanceled || stoppedEarly {
		t.Fatalf("早停数量 0 时不应早停: canceled=%v early=%v", wasCanceled, stoppedEarly)
	}
	results := officialSessionResults(session)
	if len(results) != 5 {
		t.Fatalf("结果数量 = %d, want 5", len(results))
	}
	for _, res := range results {
		if res.DataCenter != "HKG" {
			t.Fatalf("非目标数据中心结果被保留: %#v", res)
		}
	}
	if got := atomic.LoadInt64(&calls); got != 10 {
		t.Fatalf("扫描次数 = %d, want 10", got)
	}
}

func TestOfficialScanInsufficientMatchesCompletes(t *testing.T) {
	ips := officialCandidateIPs(5)
	dcByIP := map[string]string{
		ips[0]: "HKG",
		ips[1]: "LAX",
		ips[2]: "HKG",
		ips[3]: "LAX",
		ips[4]: "LAX",
	}
	var calls int64
	session, capture := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(context.Background(), session, ips, 3, 443, 500, "HKG", 20, officialMixedScanner(dcByIP, &calls))
	if wasCanceled || stoppedEarly {
		t.Fatalf("匹配不足时应正常结束: canceled=%v early=%v", wasCanceled, stoppedEarly)
	}
	if results := officialSessionResults(session); len(results) != 2 {
		t.Fatalf("结果数量 = %d, want 2", len(results))
	}
	officialDCListFromMessages(t, capture.ofType("scan_complete_wait_dc"))
	if errors := capture.ofType("error"); len(errors) != 0 {
		t.Fatalf("匹配不足不应报错: %#v", errors)
	}
}

func TestOfficialScanZeroMatchesReportsTargetDC(t *testing.T) {
	ips := officialCandidateIPs(5)
	dcByIP := make(map[string]string, len(ips))
	for _, ip := range ips {
		dcByIP[ip] = "LAX"
	}
	var calls int64
	session, capture := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(context.Background(), session, ips, 3, 443, 500, "HKG", 20, officialMixedScanner(dcByIP, &calls))
	if wasCanceled || stoppedEarly {
		t.Fatalf("wasCanceled=%v stoppedEarly=%v, want false/false", wasCanceled, stoppedEarly)
	}
	if results := officialSessionResults(session); len(results) != 0 {
		t.Fatalf("结果数量 = %d, want 0", len(results))
	}
	if !capture.contains("error", "未在数据中心 HKG 找到可用 IP") {
		t.Fatalf("缺少目标数据中心未命中提示: %#v", capture.ofType("error"))
	}
	if messages := capture.ofType("scan_complete_wait_dc"); len(messages) != 0 {
		t.Fatalf("零结果不应进入数据中心选择: %#v", messages)
	}
}

func TestOfficialScanCanceledAtStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls int64
	session, capture := newOfficialTestSession()

	wasCanceled, stoppedEarly := runOfficialScanCandidates(ctx, session, officialCandidateIPs(5), 2, 443, 500, "HKG", 20, officialStubScanner("HKG", &calls))
	if !wasCanceled || stoppedEarly {
		t.Fatalf("wasCanceled=%v stoppedEarly=%v, want true/false", wasCanceled, stoppedEarly)
	}
	if got := atomic.LoadInt64(&calls); got != 0 {
		t.Fatalf("取消后不应继续扫描: %d", got)
	}
	if !capture.contains("log", "扫描任务已终止") {
		t.Fatalf("缺少终止日志: %#v", capture.ofType("log"))
	}
	if errors := capture.ofType("error"); len(errors) != 0 {
		t.Fatalf("取消不应报目标数据中心错误: %#v", errors)
	}
}

func TestNormalizeOfficialTargetDC(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"  ":      "",
		"hkg":     "HKG",
		" Hkg ":   "HKG",
		"lax":     "LAX",
		"sjc-1":   "SJC-1",
		"  nrt  ": "NRT",
	}
	for input, want := range cases {
		if got := normalizeOfficialTargetDC(input); got != want {
			t.Fatalf("normalizeOfficialTargetDC(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveOfficialStopCount(t *testing.T) {
	if got := resolveOfficialStopCount(nil); got != defaultOfficialStopCount {
		t.Fatalf("缺省早停数量 = %d, want %d", got, defaultOfficialStopCount)
	}
	cases := map[int]int{-5: defaultOfficialStopCount, -1: defaultOfficialStopCount, 0: 0, 1: 1, 20: 20}
	for input, want := range cases {
		value := input
		if got := resolveOfficialStopCount(&value); got != want {
			t.Fatalf("resolveOfficialStopCount(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestOfficialStopConfigDefaults(t *testing.T) {
	def := defaultCLIFileConfig()
	if def.OffStop == nil || *def.OffStop != defaultOfficialStopCount {
		t.Fatalf("默认 offstop = %v, want %d", def.OffStop, defaultOfficialStopCount)
	}

	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	raw := map[string]any{"config": map[string]any{"mode": "official"}}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := loadScheduledCLIConfig(path)
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	if cfg.offStop != defaultOfficialStopCount {
		t.Fatalf("缺省调度 offstop = %d, want %d", cfg.offStop, defaultOfficialStopCount)
	}

	zero := 0
	cfgFile := defaultCLIFileConfig()
	cfgFile.OffStop = &zero
	writeScheduleConfigFile(t, path, cfgFile)
	cfg, err = loadScheduledCLIConfig(path)
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	if cfg.offStop != 0 {
		t.Fatalf("显式 0 调度 offstop = %d, want 0", cfg.offStop)
	}

	seven := 7
	cfgFile = defaultCLIFileConfig()
	cfgFile.OffStop = &seven
	writeScheduleConfigFile(t, path, cfgFile)
	cfg, err = loadScheduledCLIConfig(path)
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	if cfg.offStop != 7 {
		t.Fatalf("显式 7 调度 offstop = %d, want 7", cfg.offStop)
	}
}

func TestOfficialStopHelpEntries(t *testing.T) {
	foundHelp := false
	for _, help := range buildCLIConfigHelp() {
		if help.Name != "offstop" {
			continue
		}
		foundHelp = true
		if help.Default != "20" {
			t.Fatalf("offstop 帮助默认值 = %q, want 20", help.Default)
		}
	}
	if !foundHelp {
		t.Fatal("配置帮助缺少 offstop")
	}
	foundFlag := false
	for _, flagInfo := range cliOfficialFlags {
		if flagInfo.name == "offstop" && flagInfo.defaultValue == "20" {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Fatal("官方参数说明缺少 offstop 默认值 20")
	}
}
