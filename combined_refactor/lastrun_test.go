package main

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogThrottlePassesDistinctLines(t *testing.T) {
	var th logThrottle
	if out := th.pass("开始扫描：100 条记录"); len(out) != 1 || out[0] != "开始扫描：100 条记录" {
		t.Fatalf("首条日志应原样发出, got %v", out)
	}
	if out := th.pass("扫描完成"); len(out) != 1 || out[0] != "扫描完成" {
		t.Fatalf("不同文本应原样发出, got %v", out)
	}
}

func TestLogThrottleMergesRepeatedLines(t *testing.T) {
	var th logThrottle
	th.pass("正在等待候选池")
	for i := 0; i < 5; i++ {
		if out := th.pass("正在等待候选池"); len(out) != 0 {
			t.Fatalf("重复日志应被吞掉, got %v", out)
		}
	}
	out := th.pass("扫描完成")
	if len(out) != 2 {
		t.Fatalf("切换文本时应先补发合并说明, got %v", out)
	}
	if !strings.Contains(out[0], "共出现 6 次") || !strings.Contains(out[0], "正在等待候选池") {
		t.Errorf("合并说明应包含原文与次数, got %q", out[0])
	}
	if out[1] != "扫描完成" {
		t.Errorf("新文本应原样发出, got %q", out[1])
	}
}

func TestLogThrottleEmitsMergedAtLimit(t *testing.T) {
	var th logThrottle
	th.pass("刷屏日志")
	for i := 0; i < logThrottleMergeLimit-2; i++ {
		if out := th.pass("刷屏日志"); len(out) != 0 {
			t.Fatalf("未达上限前不应发出, got %v", out)
		}
	}
	out := th.pass("刷屏日志")
	if len(out) != 1 || !strings.Contains(out[0], "相同日志已合并") {
		t.Fatalf("达到上限应发出合并说明, got %v", out)
	}
}

func TestLogThrottleKeepsEmptyLines(t *testing.T) {
	var th logThrottle
	if out := th.pass(""); len(out) != 1 {
		t.Fatalf("空文本不应被合并逻辑吞掉, got %v", out)
	}
}

func TestLogThrottleConcurrent(t *testing.T) {
	var th logThrottle
	var wg sync.WaitGroup
	emitted := make(chan string, 1000)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				for _, out := range th.pass("同一行") {
					emitted <- out
				}
			}
		}()
	}
	wg.Wait()
	close(emitted)
	count := 0
	for range emitted {
		count++
	}
	if count < 1 || count > 10 {
		t.Errorf("并发合并后应至少发出一条且不超过 goroutine 数, got %d", count)
	}
}

func TestRecordLastRunAndPayload(t *testing.T) {
	lastRunMutex.Lock()
	prev := lastRunInfo
	lastRunInfo = nil
	lastRunMutex.Unlock()
	defer func() {
		lastRunMutex.Lock()
		lastRunInfo = prev
		lastRunMutex.Unlock()
	}()

	if lastRunPayload() != nil {
		t.Fatalf("无记录时 payload 应为 nil")
	}
	started := time.Now().Add(-2 * time.Second)
	recordLastRun("official", "官方优选扫描", started, true, 42, "R2 已上传 20 条")
	info := lastRunPayload()
	if info == nil {
		t.Fatalf("记录后 payload 不应为 nil")
	}
	if info.Mode != "official" || info.Label != "官方优选扫描" || !info.Success || info.Results != 42 {
		t.Errorf("摘要字段不正确: %+v", info)
	}
	if info.DurationMs < 1500 {
		t.Errorf("耗时计算不正确: %d", info.DurationMs)
	}
	if info.UploadNote != "R2 已上传 20 条" {
		t.Errorf("上传说明不正确: %q", info.UploadNote)
	}

	recordLastRun("nsb", "非标优选", time.Now(), false, 0, "R2 上传未启用")
	info = lastRunPayload()
	if info.Mode != "nsb" || info.Success || info.UploadNote != "R2 上传未启用" {
		t.Errorf("最新记录应覆盖旧记录: %+v", info)
	}

	updateLastRunUploadNote("R2 已上传 5 条")
	if got := lastRunPayload().UploadNote; got != "R2 已上传 5 条" {
		t.Errorf("更新上传说明失败, got %q", got)
	}
}

func TestSessionUploadNote(t *testing.T) {
	session := &appSession{}
	if got := session.uploadNoteValue(); got != "" {
		t.Fatalf("初始上传说明应为空, got %q", got)
	}
	session.setUploadNote("R2 上传中…")
	if got := session.uploadNoteValue(); got != "R2 上传中…" {
		t.Errorf("上传说明设置失败, got %q", got)
	}
	session.setUploadNote("R2 已上传 3 条")
	if got := session.uploadNoteValue(); got != "R2 已上传 3 条" {
		t.Errorf("上传说明覆盖失败, got %q", got)
	}
}

func TestSessionFinalResultCount(t *testing.T) {
	session := &appSession{}
	if got := session.finalResultCount(); got != 0 {
		t.Errorf("空会话结果数应为 0, got %d", got)
	}
	session.scanResults = []ScanResult{{IP: "1.1.1.1"}}
	session.testResults = []TestResult{{IP: "1.1.1.1"}}
	if got := session.finalResultCount(); got != 1 {
		t.Errorf("有详细测试时应优先返回详细测试行数, got %d", got)
	}
	session.testResults = nil
	if got := session.finalResultCount(); got != 1 {
		t.Errorf("无详细测试时应返回扫描结果行数, got %d", got)
	}
	session.scanResults = nil
	session.nsbRows = [][]string{{"a"}, {"b"}}
	if got := session.finalResultCount(); got != 2 {
		t.Errorf("非标结果行数应为 2, got %d", got)
	}
}
