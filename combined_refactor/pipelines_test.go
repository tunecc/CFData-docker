package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOfficialFlowScanOnlyInvokesOnDone(t *testing.T) {
	session := &appSession{}
	done := make(chan error, 1)
	called := false
	opts := officialFlowOptions{
		fromLibrary: true, // 空库列表：离线快速返回，不触网
		libraryIPs:  nil,
		autoSpeed:   false, // 仅扫描
		onDone: func(err error) {
			called = true
			done <- err
		},
	}
	err := runOfficialTaskFlow(context.Background(), session, opts)
	if err != nil {
		t.Fatalf("仅扫描分支应无错误: %v", err)
	}
	if !called {
		t.Fatal("onDone 应被调用")
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("onDone 错误不符: %v", e)
		}
	default:
		t.Fatal("onDone 未投递结果")
	}
}

func TestOfficialFlowCanceledContext(t *testing.T) {
	session := &appSession{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	opts := officialFlowOptions{
		fromLibrary: true,
		autoSpeed:   false,
		onDone:      func(err error) { done <- err },
	}
	err := runOfficialTaskFlow(ctx, session, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消的 ctx 应返回 context.Canceled，得到 %v", err)
	}
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatalf("onDone 错误不符: %v", e)
		}
	default:
		t.Fatal("onDone 未投递结果")
	}
}

func TestNSBFlowMissingInput(t *testing.T) {
	session := &appSession{}
	done := make(chan error, 1)
	opts := nsbFlowOptions{
		onDone: func(err error) { done <- err },
	}
	err := runNSBTaskFlow(context.Background(), session, opts)
	if err == nil || !strings.Contains(err.Error(), "IP 文件或网络 URL") {
		t.Fatalf("缺少输入源应报错，得到 %v", err)
	}
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("onDone 应携带错误")
		}
	default:
		t.Fatal("onDone 未投递结果")
	}
}

func TestNSBFlowCanceledContext(t *testing.T) {
	session := &appSession{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	opts := nsbFlowOptions{
		fileContent: "1.2.3.4 443",
		onDone:      func(err error) { done <- err },
	}
	err := runNSBTaskFlow(ctx, session, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消的 ctx 应返回 context.Canceled，得到 %v", err)
	}
}

func TestNSBFlowFilePriorityOverURL(t *testing.T) {
	// 修复回归：file 与 URL 同设时旧实现优先文件（loadNSBInput file 分支在前）。
	// 通过不存在的 file 触发读文件错误（而非发起 URL 拉取）来验证优先级。
	session := &appSession{}
	done := make(chan error, 1)
	opts := nsbFlowOptions{
		fileName:  filepath.Join(t.TempDir(), "missing.txt"),
		sourceURL: "https://example.invalid/list.txt",
		onDone:    func(err error) { done <- err },
	}
	err := runNSBTaskFlow(context.Background(), session, opts)
	if err == nil || !strings.Contains(err.Error(), "missing.txt") {
		t.Fatalf("应优先读文件并报文件错误，得到 %v", err)
	}
}

func TestOfficialFlowNormalizesTargetDC(t *testing.T) {
	// 修复回归：调度路径 dc 大写规范化（对齐旧管线 normalizeOfficialTargetDC）。
	// 库输入空列表离线返回后，flow 内部 pickBest 不会触发；这里只验证
	// normalize 后的分支不 panic 且 onDone 正常投递。
	session := &appSession{}
	done := make(chan error, 1)
	opts := officialFlowOptions{
		fromLibrary: true,
		targetDC:    "hkg",
		autoSpeed:   false,
		onDone:      func(err error) { done <- err },
	}
	_ = runOfficialTaskFlow(context.Background(), session, opts)
	select {
	case <-done:
	default:
		t.Fatal("onDone 未投递")
	}
}

func TestScheduledFailureWritesFailureHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CFDATA_HISTORY_DIR", dir)
	session := &appSession{}
	session.scheduleRun = true
	runDone := make(chan struct{})
	session.startTaskNamed("调度任务", "official", nil, func(ctx context.Context, session *appSession) {
		defer close(runDone)
		session.setFlowFailure("官方模式未发现有效 IP")
	})
	<-runDone
	// startTaskNamed 的收尾 defer 在 run 返回后执行；轮询等待失败历史落盘
	deadline := time.Now().Add(3 * time.Second)
	var found *taskHistoryRecord
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			var record taskHistoryRecord
			if json.Unmarshal(data, &record) == nil && record.FailureSummary != "" {
				found = &record
			}
		}
		if found != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if found == nil {
		t.Fatal("调度失败应写入含 failureSummary 的失败历史（A2 回归）")
	}
}
