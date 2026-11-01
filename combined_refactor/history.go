package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 任务历史记录（学习 subs-check-pro「一份分析报告多处复用」的设计模式，
// 见 docs/analyses/subs-check-pro-learnings.md 建议①②⑦）：任务完成后把
// 统计摘要落盘 /history/，重启后从 latest 恢复分析视图；记录是统计摘要
// 加 Top 结果，不是全量结果表（全量导出已有 CSV/R2 路径）。

const (
	taskHistorySchema      = "cfdata.task-history.v1"
	taskHistoryDirName     = "history"
	taskHistoryLatestName  = "latest.json"
	taskHistoryListLimit   = 200
	taskHistoryTopResults  = 20
	historyFileNamePattern = `^(\d{14})-(official|nsb)\.json$`
)

var historyFileNameRE = regexp.MustCompile(historyFileNamePattern)

type historyTopItem struct {
	IP         string `json:"ip"`
	Port       int    `json:"port,omitempty"`
	DataCenter string `json:"dataCenter,omitempty"`
	Latency    string `json:"latency,omitempty"`
	Speed      string `json:"speed,omitempty"`
}

type taskHistoryRecord struct {
	Schema             string           `json:"schema"`
	Mode               string           `json:"mode"`
	Label              string           `json:"label,omitempty"`
	StartedAt          time.Time        `json:"startedAt"`
	FinishedAt         time.Time        `json:"finishedAt"`
	DurationSec        int              `json:"durationSec"`
	ScanCount          int              `json:"scanCount"`
	TestCount          int              `json:"testCount"`
	NSBCount           int              `json:"nsbCount"`
	ResultCount        int              `json:"resultCount"`
	QualifiedCount     int              `json:"qualifiedCount"`
	FailureSummary     string           `json:"failureSummary,omitempty"`
	DCDistribution     map[string]int   `json:"dcDistribution,omitempty"`
	RegionDistribution map[string]int   `json:"regionDistribution,omitempty"`
	TopResults         []historyTopItem `json:"topResults,omitempty"`
}

type historyListItem struct {
	File           string `json:"file"`
	Mode           string `json:"mode"`
	StartedAt      string `json:"startedAt"`
	ResultCount    int    `json:"resultCount"`
	QualifiedCount int    `json:"qualifiedCount"`
	DurationSec    int    `json:"durationSec"`
}

// historyDir 返回历史目录：优先环境变量 CFDATA_HISTORY_DIR（测试隔离与
// 部署自定义），默认为配置文件同级的 history/ 子目录。
func historyDir() string {
	if override := strings.TrimSpace(os.Getenv("CFDATA_HISTORY_DIR")); override != "" {
		return override
	}
	return filepath.Join(filepath.Dir(defaultCLIConfigPath()), taskHistoryDirName)
}

func historyRecordFileName(now time.Time, mode string) string {
	return fmt.Sprintf("%s-%s.json", now.UTC().Format("20060102150405"), mode)
}

// isValidHistoryFileName 校验文件名由本服务生成，防路径穿越。
func isValidHistoryFileName(name string) bool {
	return historyFileNameRE.MatchString(filepath.Base(name)) && !strings.ContainsAny(name, "/\\")
}

func (r *taskHistoryRecord) validate() error {
	if r.Schema != taskHistorySchema {
		return fmt.Errorf("未知历史记录 schema: %q", r.Schema)
	}
	if r.Mode != "official" && r.Mode != "nsb" {
		return fmt.Errorf("未知历史记录 mode: %q", r.Mode)
	}
	return nil
}

// parseSpeedMBs 从测速文本（如 "12.34 MB/s"）提取数值部分。
func parseSpeedMBs(speed string) float64 {
	fields := strings.Fields(strings.TrimSpace(speed))
	if len(fields) == 0 {
		return 0
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return value
}

func (s *appSession) scanFailureSummaryValue() string {
	s.scanMutex.Lock()
	defer s.scanMutex.Unlock()
	return s.scanFailureSummary
}

func (s *appSession) setScanFailureSummary(text string) {
	s.scanMutex.Lock()
	defer s.scanMutex.Unlock()
	s.scanFailureSummary = text
}

// buildTaskHistoryRecord 从会话当前结果构建统计记录。
func buildTaskHistoryRecord(s *appSession, mode, label string, startedAt, finishedAt time.Time, failureSummary string) taskHistoryRecord {
	s.scanMutex.Lock()
	scanCopy := append([]ScanResult(nil), s.scanResults...)
	scanFailure := s.scanFailureSummary
	s.scanMutex.Unlock()
	s.testMutex.Lock()
	testCopy := append([]TestResult(nil), s.testResults...)
	sortOfficialTestResults(testCopy) // 原地按丢包/延迟排序
	s.testMutex.Unlock()
	s.nsbMutex.Lock()
	nsbCount := len(s.nsbRows)
	s.nsbMutex.Unlock()

	record := taskHistoryRecord{
		Schema:      taskHistorySchema,
		Mode:        mode,
		Label:       label,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		DurationSec: int(finishedAt.Sub(startedAt).Seconds() + 0.5),
		ScanCount:   len(scanCopy),
		TestCount:   len(testCopy),
		NSBCount:    nsbCount,
	}
	record.ResultCount = len(testCopy)
	if record.ResultCount == 0 {
		record.ResultCount = len(scanCopy)
	}
	if record.ResultCount == 0 {
		record.ResultCount = nsbCount
	}

	// 合格数口径（brief D2）：官方=测速文本数值达到配置 SpeedMin（未配置按 >0）；
	// 非标=扫描合格行数
	switch mode {
	case "official":
		speedMin := 0.0
		if cfg, err := readScheduleConfigFile(defaultCLIConfigPath()); err == nil {
			speedMin = cfg.SpeedMin
		}
		for _, res := range testCopy {
			speed := parseSpeedMBs(res.Speed)
			if speedMin > 0 {
				if speed >= speedMin {
					record.QualifiedCount++
				}
			} else if speed > 0 {
				record.QualifiedCount++
			}
		}
	case "nsb":
		record.QualifiedCount = nsbCount
	}

	record.FailureSummary = strings.TrimSpace(failureSummary)
	if record.FailureSummary == "" {
		record.FailureSummary = strings.TrimSpace(scanFailure)
	}

	// DC / 地区分布：优先详细测试，其次扫描结果
	dcDistribution := map[string]int{}
	regionDistribution := map[string]int{}
	addDistribution := func(dc, region string) {
		if dc = strings.TrimSpace(dc); dc != "" {
			dcDistribution[dc]++
		}
		if region = strings.TrimSpace(region); region != "" {
			regionDistribution[region]++
		}
	}
	if len(testCopy) > 0 {
		for _, res := range testCopy {
			addDistribution(res.DataCenter, res.Region)
		}
	} else {
		for _, res := range scanCopy {
			addDistribution(res.DataCenter, res.Region)
		}
	}
	if len(dcDistribution) > 0 {
		record.DCDistribution = dcDistribution
	}
	if len(regionDistribution) > 0 {
		record.RegionDistribution = regionDistribution
	}

	// Top 结果：官方按延迟升序取前 N；非标无结构化结果则留空
	if len(testCopy) > 0 {
		limit := taskHistoryTopResults
		if len(testCopy) < limit {
			limit = len(testCopy)
		}
		for _, res := range testCopy[:limit] {
			record.TopResults = append(record.TopResults, historyTopItem{
				IP:         res.IP,
				Port:       res.Port,
				DataCenter: res.DataCenter,
				Latency:    fmt.Sprintf("%dms", int(res.AvgLatency/time.Millisecond)),
				Speed:      res.Speed,
			})
		}
	} else if len(scanCopy) > 0 {
		sorted := append([]ScanResult(nil), scanCopy...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].TCPDuration < sorted[j].TCPDuration })
		limit := taskHistoryTopResults
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for _, res := range sorted[:limit] {
			record.TopResults = append(record.TopResults, historyTopItem{
				IP:         res.IP,
				Port:       res.Port,
				DataCenter: res.DataCenter,
				Latency:    res.LatencyStr,
			})
		}
	}
	return record
}

// persistTaskHistory 构建并落盘历史记录，成功后向前端推送 history_analysis。
// 失败只记日志，不影响任务终态。
func persistTaskHistory(s *appSession, mode, label string, startedAt time.Time, failureSummary string) {
	finishedAt := time.Now()
	record := buildTaskHistoryRecord(s, mode, label, startedAt, finishedAt, failureSummary)
	fileName, err := writeTaskHistoryRecord(record)
	if err != nil {
		fmt.Printf("[history] 历史记录写入失败: %v\n", err)
		return
	}
	s.sendWSMessage("history_analysis", map[string]interface{}{
		"record": record,
		"file":   fileName,
	})
}

func writeTaskHistoryRecord(record taskHistoryRecord) (string, error) {
	if err := record.validate(); err != nil {
		return "", err
	}
	dir := historyDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	fileName := historyRecordFileName(record.FinishedAt, record.Mode)
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", err
	}
	if err := atomicWriteFile(filepath.Join(dir, fileName), data, 0644); err != nil {
		return "", err
	}
	latest := map[string]string{"file": fileName, "schema": record.Schema}
	latestData, err := json.MarshalIndent(latest, "", "  ")
	if err != nil {
		return fileName, err
	}
	if err := atomicWriteFile(filepath.Join(dir, taskHistoryLatestName), latestData, 0644); err != nil {
		return fileName, err
	}
	return fileName, nil
}

func readHistoryFile(name string) (*taskHistoryRecord, error) {
	if !isValidHistoryFileName(name) {
		return nil, fmt.Errorf("非法历史文件名: %q", name)
	}
	data, err := os.ReadFile(filepath.Join(historyDir(), filepath.Base(name)))
	if err != nil {
		return nil, err
	}
	var record taskHistoryRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("解析历史记录失败 %s: %w", name, err)
	}
	if err := record.validate(); err != nil {
		return nil, err
	}
	return &record, nil
}

func readLatestHistory() (*taskHistoryRecord, string, error) {
	data, err := os.ReadFile(filepath.Join(historyDir(), taskHistoryLatestName))
	if err != nil {
		return nil, "", err
	}
	var latest struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(data, &latest); err != nil {
		return nil, "", err
	}
	record, err := readHistoryFile(latest.File)
	if err != nil {
		return nil, latest.File, err
	}
	return record, latest.File, nil
}

// listHistoryRecords 返回倒序列表（最多 limit 条），缺失字段的损坏文件跳过。
func listHistoryRecords(limit int) ([]historyListItem, error) {
	dir := historyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []historyListItem{}, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == taskHistoryLatestName || !isValidHistoryFileName(name) {
			continue
		}
		names = append(names, name)
	}
	// 文件名以 UTC 时间戳开头，倒序即最新在前
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if limit <= 0 || limit > taskHistoryListLimit {
		limit = taskHistoryListLimit
	}
	items := make([]historyListItem, 0, len(names))
	for _, name := range names {
		if len(items) >= limit {
			break
		}
		record, err := readHistoryFile(name)
		if err != nil {
			continue
		}
		items = append(items, historyListItem{
			File:           name,
			Mode:           record.Mode,
			StartedAt:      record.StartedAt.Format(time.RFC3339),
			ResultCount:    record.ResultCount,
			QualifiedCount: record.QualifiedCount,
			DurationSec:    record.DurationSec,
		})
	}
	return items, nil
}

func deleteHistoryFile(name string) error {
	if !isValidHistoryFileName(name) {
		return fmt.Errorf("非法历史文件名: %q", name)
	}
	return os.Remove(filepath.Join(historyDir(), filepath.Base(name)))
}

func clearHistoryFiles() (int, error) {
	dir := historyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isValidHistoryFileName(name) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err == nil {
			removed++
		}
	}
	// latest 指针指向的记录已删除，一并清理
	_ = os.Remove(filepath.Join(dir, taskHistoryLatestName))
	return removed, nil
}

// sendLatestHistoryToSession 在无任务运行时把 latest 记录下发给新连接（重启恢复）。
func sendLatestHistoryToSession(s *appSession) {
	if anyTaskRunning() {
		return
	}
	record, file, err := readLatestHistory()
	if err != nil || record == nil {
		return
	}
	s.sendWSMessage("history_analysis", map[string]interface{}{
		"record":      record,
		"file":        file,
		"fromHistory": true,
	})
}

// flowFailure 记录本次任务的管线错误（非用户停止），供任务收尾区分成败。
func (s *appSession) setFlowFailure(text string) {
	s.flowFailureMutex.Lock()
	defer s.flowFailureMutex.Unlock()
	s.flowFailure = text
}

func (s *appSession) flowFailureValue() string {
	s.flowFailureMutex.Lock()
	defer s.flowFailureMutex.Unlock()
	return s.flowFailure
}
