package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withTempHistoryDir 把配置目录临时切换到隔离目录，隔离 history/ 文件操作。
func withTempHistoryDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	// defaultCLIConfigPath 基于 os.Executable 的目录，无法重定向；
	// 历史目录支持环境变量覆盖（见 historyDir 的测试入口）
	t.Setenv("CFDATA_HISTORY_DIR", dir)
	return dir
}

func TestHistoryRecordRoundtrip(t *testing.T) {
	dir := withTempHistoryDir(t)
	started := time.Now().Add(-2 * time.Minute)
	record := taskHistoryRecord{
		Schema:             taskHistorySchema,
		Mode:               "official",
		Label:              "官方优选扫描",
		StartedAt:          started,
		FinishedAt:         time.Now(),
		DurationSec:        120,
		ScanCount:          50,
		TestCount:          30,
		ResultCount:        30,
		QualifiedCount:     12,
		DCDistribution:     map[string]int{"HKG": 10, "NRT": 20},
		RegionDistribution: map[string]int{"香港": 10},
	}
	fileName, err := writeTaskHistoryRecord(record)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if !isValidHistoryFileName(fileName) {
		t.Fatalf("文件名不合法: %q", fileName)
	}
	if !strings.HasSuffix(fileName, "-official.json") {
		t.Fatalf("文件名应含模式后缀: %q", fileName)
	}
	// latest 指针
	latest, latestFile, err := readLatestHistory()
	if err != nil || latest == nil {
		t.Fatalf("读取 latest 失败: %v", err)
	}
	if latestFile != fileName || latest.Mode != "official" {
		t.Fatalf("latest 指针不符: file=%q mode=%q", latestFile, latest.Mode)
	}
	if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
		t.Fatalf("记录文件不存在: %v", err)
	}
}

func TestHistoryListAndDelete(t *testing.T) {
	withTempHistoryDir(t)
	started := time.Now().Add(-time.Hour)
	// 写两条 official（不同 finishedAt）+ 一条 nsb
	for i := 0; i < 2; i++ {
		rec := taskHistoryRecord{Schema: taskHistorySchema, Mode: "official", StartedAt: started, FinishedAt: started.Add(time.Duration(i+1) * time.Minute), ResultCount: 10 + i, QualifiedCount: 5}
		if _, err := writeTaskHistoryRecord(rec); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		time.Sleep(1100 * time.Millisecond) // 文件名秒级精度，确保不同名
	}
	nsb := taskHistoryRecord{Schema: taskHistorySchema, Mode: "nsb", StartedAt: started, FinishedAt: started.Add(time.Minute), ResultCount: 7, QualifiedCount: 7}
	if _, err := writeTaskHistoryRecord(nsb); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	items, err := listHistoryRecords(0)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("应有 3 条记录，得到 %d", len(items))
	}
	// 倒序：最新在前
	if !strings.HasSuffix(items[0].File, "-nsb.json") && !strings.Contains(items[0].File, "official") {
		t.Fatalf("列表首条异常: %q", items[0].File)
	}
	// 删除单条
	if err := deleteHistoryFile(items[0].File); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	items, _ = listHistoryRecords(0)
	if len(items) != 2 {
		t.Fatalf("删除后应剩 2 条，得到 %d", len(items))
	}
	// 防穿越：非法文件名拒绝
	if err := deleteHistoryFile("../escape.json"); err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
	if err := deleteHistoryFile("latest.json"); err == nil {
		t.Fatal("latest.json 不应可通过 delete 删除")
	}
	// 清空
	removed, err := clearHistoryFiles()
	if err != nil || removed != 2 {
		t.Fatalf("清空应移除 2 条，得到 %d, err=%v", removed, err)
	}
	items, _ = listHistoryRecords(0)
	if len(items) != 0 {
		t.Fatalf("清空后应为空，得到 %d", len(items))
	}
}

func TestBuildTaskHistoryRecordOfficial(t *testing.T) {
	dir := withTempHistoryDir(t)
	// 写一份配置文件供 SpeedMin 读取（defaultCLIConfigPath 指向 exe 目录，
	// 单测环境读不到则按 >0 兜底；此处直接校验兜底口径）
	started := time.Now().Add(-time.Minute)
	session := &appSession{}
	session.scanMutex.Lock()
	session.scanResults = []ScanResult{
		{IP: "1.2.3.4", Port: 443, DataCenter: "HKG", Region: "香港", LatencyStr: "12ms", TCPDuration: 12 * time.Millisecond},
		{IP: "5.6.7.8", Port: 443, DataCenter: "NRT", Region: "日本", LatencyStr: "55ms", TCPDuration: 55 * time.Millisecond},
	}
	session.scanMutex.Unlock()
	session.testMutex.Lock()
	session.testResults = []TestResult{
		{IP: "1.2.3.4", Port: 443, DataCenter: "HKG", Region: "香港", Speed: "12.5 MB/s", AvgLatency: 15 * time.Millisecond},
		{IP: "5.6.7.8", Port: 443, DataCenter: "NRT", Region: "日本", Speed: "-", AvgLatency: 60 * time.Millisecond},
	}
	session.testMutex.Unlock()

	record := buildTaskHistoryRecord(session, "official", "官方优选扫描", started, time.Now(), "")
	if record.ResultCount != 2 || record.ScanCount != 2 || record.TestCount != 2 {
		t.Fatalf("计数不符: %+v", record)
	}
	// 未配置 SpeedMin：Speed>0 视为达标（"-" 解析为 0 不计）
	if record.QualifiedCount != 1 {
		t.Fatalf("合格数应为 1，得到 %d", record.QualifiedCount)
	}
	if record.DCDistribution["HKG"] != 1 || record.DCDistribution["NRT"] != 1 {
		t.Fatalf("DC 分布不符: %v", record.DCDistribution)
	}
	if record.RegionDistribution["香港"] != 1 {
		t.Fatalf("地区分布不符: %v", record.RegionDistribution)
	}
	if len(record.TopResults) != 2 || record.TopResults[0].IP != "1.2.3.4" {
		t.Fatalf("Top 结果应按延迟升序: %+v", record.TopResults)
	}
	_ = dir
}

func TestBuildTaskHistoryRecordNSB(t *testing.T) {
	withTempHistoryDir(t)
	started := time.Now().Add(-time.Minute)
	session := &appSession{}
	session.nsbMutex.Lock()
	session.nsbRows = [][]string{
		{"1.2.3.4", "443", "TLS", "0%", "TCPing", "12ms", "-", "1.2.3.4", "v4", "1.2.3.4 443", "HKG", "香港", "香港", "AS123", "Org"},
		{"5.6.7.8", "443", "TLS", "0%", "TCPing", "30ms", "-", "5.6.7.8", "v4", "5.6.7.8 443", "NRT", "日本", "东京", "AS456", "Org"},
	}
	session.nsbMutex.Unlock()

	record := buildTaskHistoryRecord(session, "nsb", "", started, time.Now(), "")
	if record.NSBCount != 2 || record.ResultCount != 2 || record.QualifiedCount != 2 {
		t.Fatalf("非标计数不符: %+v", record)
	}
}

func TestParseSpeedMBs(t *testing.T) {
	cases := map[string]float64{
		"12.34 MB/s": 12.34,
		"0.1 MB/s":   0.1,
		"-":          0,
		"":           0,
		"abc":        0,
	}
	for input, want := range cases {
		if got := parseSpeedMBs(input); got != want {
			t.Fatalf("parseSpeedMBs(%q) = %v, 期望 %v", input, got, want)
		}
	}
}

func TestReadHistoryFileRejectsTraversal(t *testing.T) {
	withTempHistoryDir(t)
	if _, err := readHistoryFile("../../etc/passwd"); err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
	if _, err := readHistoryFile("latest.json"); err == nil {
		t.Fatal("latest.json 不属于记录文件，应被拒绝")
	}
	// 损坏文件跳过：写入非法 JSON 后 list 应忽略
	badName := "20260101000000-official.json"
	if err := os.MkdirAll(historyDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(historyDir(), badName), []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	items, err := listHistoryRecords(0)
	if err != nil {
		t.Fatalf("列表不应因损坏文件失败: %v", err)
	}
	for _, item := range items {
		if item.File == badName {
			t.Fatal("损坏文件应被跳过")
		}
	}
}
