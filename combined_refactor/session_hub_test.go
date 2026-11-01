package main

import (
	"context"
	"testing"
	"time"
)

// waitHubIdle 等待全局任务互斥释放，避免用例间相互干扰。
func waitHubIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for anyTaskRunning() {
		if time.Now().After(deadline) {
			t.Fatal("全局任务未在 5 秒内结束")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// 启动处理器在 startTaskNamed 之前写入 r2AutoUpload；beginTask 的任务级复位
// 必须不能把刚写入的启动参数清掉（回归防护：resetReplayState 曾误清该字段，
// 导致 Web 端 R2 自动上传整体失效）。
func TestHubSessionR2AutoUploadSurvivesTaskStart(t *testing.T) {
	waitHubIdle(t)
	s := newHubSession()
	s.r2AutoUpload = true
	done := make(chan struct{})
	s.startTaskNamed("回归测试", "official", nil, func(ctx context.Context, sess *appSession) {
		defer close(done)
		if !sess.r2AutoUpload {
			t.Error("任务体内读不到 r2AutoUpload=true：beginTask 的复位覆盖了启动参数")
		}
	})
	<-done
	waitHubIdle(t)
}

// 上一任务的 R2 上传备注必须在新任务开始时清空，避免串入下一任务的 last_run 摘要。
func TestHubSessionUploadNoteResetOnNewTask(t *testing.T) {
	waitHubIdle(t)
	s := newHubSession()
	s.setUploadNote("上一任务的 R2 上传备注")
	release := make(chan struct{})
	started := make(chan struct{})
	s.startTaskNamed("回归测试", "official", nil, func(ctx context.Context, sess *appSession) {
		if note := sess.uploadNoteValue(); note != "" {
			t.Errorf("新任务开始时应清空上一任务备注，实际: %q", note)
		}
		close(started)
		<-release
	})
	<-started
	// 任务运行中写入的备注在本任务内必须可读（任务结束节点消费）
	s.setUploadNote("当前任务备注")
	if note := s.uploadNoteValue(); note != "当前任务备注" {
		t.Errorf("当前任务备注应可读，实际: %q", note)
	}
	close(release)
	waitHubIdle(t)
}
