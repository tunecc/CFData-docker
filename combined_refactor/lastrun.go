package main

import (
	"fmt"
	"sync"
	"time"
)

// lastRunSummary 保存最近一次任务的摘要。只存在服务器内存中：
// 页面刷新后通过 init_config 重新下发，进程重启后回到空状态。
type lastRunSummary struct {
	Mode       string `json:"mode"`
	Label      string `json:"label"`
	Success    bool   `json:"success"`
	EndTime    int64  `json:"endTime"`
	DurationMs int64  `json:"durationMs"`
	Results    int    `json:"results"`
	UploadNote string `json:"uploadNote"`
}

var (
	lastRunMutex sync.Mutex
	lastRunInfo  *lastRunSummary
)

func recordLastRun(mode, label string, startedAt time.Time, success bool, results int, uploadNote string) {
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	info := &lastRunSummary{
		Mode:       mode,
		Label:      label,
		Success:    success,
		EndTime:    time.Now().Unix(),
		DurationMs: time.Since(startedAt).Milliseconds(),
		Results:    results,
		UploadNote: uploadNote,
	}
	lastRunMutex.Lock()
	lastRunInfo = info
	lastRunMutex.Unlock()
}

func updateLastRunUploadNote(note string) {
	lastRunMutex.Lock()
	if lastRunInfo != nil {
		lastRunInfo.UploadNote = note
	}
	lastRunMutex.Unlock()
}

func lastRunPayload() *lastRunSummary {
	lastRunMutex.Lock()
	defer lastRunMutex.Unlock()
	if lastRunInfo == nil {
		return nil
	}
	summary := *lastRunInfo
	return &summary
}

// logThrottle 把短时间内连续重复的相同日志合并为一条带重复计数的说明，
// 避免高频逐行日志刷屏；不同文本照常发出。
type logThrottle struct {
	mu    sync.Mutex
	last  string
	count int
}

const logThrottleMergeLimit = 100

// pass 返回本次应当发出的日志文本列表；重复文本被吞掉时返回空列表。
func (t *logThrottle) pass(text string) []string {
	if text == "" {
		return []string{text}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if text != t.last {
		out := make([]string, 0, 2)
		if t.count > 1 {
			out = append(out, fmt.Sprintf("%s（以上相同日志已合并，共出现 %d 次）", t.last, t.count))
		}
		t.last = text
		t.count = 1
		out = append(out, text)
		return out
	}
	t.count++
	if t.count >= logThrottleMergeLimit {
		merged := fmt.Sprintf("%s（相同日志已合并，共出现 %d 次）", text, t.count)
		t.count = 1
		return []string{merged}
	}
	return nil
}
