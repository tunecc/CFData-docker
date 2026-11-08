package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTaskSnapshotDir 把历史目录指到临时目录，隔离 task-state.json 的读写。
func setupTaskSnapshotDir(t *testing.T) {
	t.Helper()
	t.Setenv("CFDATA_HISTORY_DIR", t.TempDir())
}

// waitForTaskTail 等待任务收尾尾段（recordLastRun/persistTaskHistory 等在
// endTask 之后执行的写盘步骤）完成，避免 t.TempDir 清理与后台写盘竞争。
// latest.json 是尾段最后写入的文件；同测试内多个任务时先 removeLatestHistory
// 清掉上一任务的 latest.json，让本等待有明确目标。
func waitForTaskTail(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(historyDir(), taskHistoryLatestName)); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("任务收尾写盘未在 2 秒内完成")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func removeLatestHistory(t *testing.T) {
	t.Helper()
	if err := os.Remove(filepath.Join(historyDir(), taskHistoryLatestName)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// runSnapshotTask 在全局任务会话上跑一个带产物（日志/扫描/测速）的短任务，
// 任务结束时 defer 会把快照落盘。
func runSnapshotTask(t *testing.T, label string, ip string) {
	t.Helper()
	waitHubIdle(t)
	done := make(chan struct{})
	taskSession.startTaskNamed(label, "official", nil, func(ctx context.Context, sess *appSession) {
		defer close(done)
		sess.emitTaskMessage("log", label+" 日志行")
		sess.emitTaskMessage("scan_result", ScanResult{IP: ip, Port: 443, LatencyStr: "10ms"})
		sess.emitTaskMessage("speed_test_result", map[string]string{ip: "12.34 MB/s"})
	})
	<-done
	waitHubIdle(t)
	waitForTaskTail(t)
}

// 任务收尾必须把回放内容完整落盘，重启后（内存清空）可重建同构恢复载荷。
func TestTaskSnapshotPersistAndRestoreRoundtrip(t *testing.T) {
	setupTaskSnapshotDir(t)
	runSnapshotTask(t, "快照往返", "1.2.3.4")

	snap, err := loadTaskDiskSnapshot()
	if err != nil {
		t.Fatalf("读取任务快照失败: %v", err)
	}
	if snap.Label != "快照往返" {
		t.Errorf("快照 label = %q, 期望 快照往返", snap.Label)
	}
	if len(snap.Logs) != 1 || snap.Logs[0] != "快照往返 日志行" {
		t.Errorf("快照日志 = %v, 期望 1 行任务日志", snap.Logs)
	}
	if len(snap.Scan) != 1 || snap.Scan[0].IP != "1.2.3.4" {
		t.Errorf("快照扫描行 = %v, 期望 1 行 1.2.3.4", snap.Scan)
	}
	if len(snap.Speed) != 1 {
		t.Errorf("快照测速行数 = %d, 期望 1", len(snap.Speed))
	}
	if snap.Snapshot.Running {
		t.Error("快照终态 Running 应为 false")
	}
	if snap.Schema != taskDiskSnapshotSchema {
		t.Errorf("快照 schema = %q, 期望 %q", snap.Schema, taskDiskSnapshotSchema)
	}

	// 重建载荷与内存回放同构：hasTask/fromDisk/日志/结果分片
	payload, chunks, ok := buildTaskRestoreFromDisk()
	if !ok {
		t.Fatal("buildTaskRestoreFromDisk 应成功")
	}
	if !payload.HasTask || !payload.FromDisk || payload.Running {
		t.Errorf("载荷标志异常: hasTask=%v fromDisk=%v running=%v", payload.HasTask, payload.FromDisk, payload.Running)
	}
	if payload.Label != "快照往返" || len(payload.Logs) != 1 {
		t.Errorf("载荷元信息异常: label=%q logs=%d", payload.Label, len(payload.Logs))
	}
	kinds := map[string]int{}
	for _, chunk := range chunks {
		kinds[chunk.Kind]++
	}
	if kinds["scan"] != 1 || kinds["speed"] != 1 {
		t.Errorf("恢复分片异常: %v", kinds)
	}
}

// 新任务开始（beginTask 清空内存回放）不得删除快照；新任务结束时覆盖为新内容。
func TestTaskSnapshotSurvivesBeginTaskAndOverwrite(t *testing.T) {
	setupTaskSnapshotDir(t)
	runSnapshotTask(t, "第一任务", "1.1.1.1")

	// 第二任务运行中：内存回放已被 beginTask 清空，但文件仍是第一任务内容
	removeLatestHistory(t)
	release := make(chan struct{})
	started := make(chan struct{})
	done := make(chan struct{})
	taskSession.startTaskNamed("第二任务", "official", nil, func(ctx context.Context, sess *appSession) {
		defer close(done)
		close(started)
		<-release
		sess.emitTaskMessage("log", "第二任务 日志行")
	})
	<-started
	snap, err := loadTaskDiskSnapshot()
	if err != nil {
		t.Fatalf("beginTask 后快照不应缺失: %v", err)
	}
	if snap.Label != "第一任务" {
		t.Errorf("beginTask 后快照 label = %q, 期望仍为 第一任务", snap.Label)
	}
	close(release)
	<-done
	waitHubIdle(t)
	waitForTaskTail(t)

	snap, err = loadTaskDiskSnapshot()
	if err != nil {
		t.Fatalf("读取覆盖后快照失败: %v", err)
	}
	if snap.Label != "第二任务" || len(snap.Logs) != 1 || snap.Logs[0] != "第二任务 日志行" {
		t.Errorf("新任务结束后快照未覆盖: label=%q logs=%v", snap.Label, snap.Logs)
	}
}

// 快照缺失、损坏或 schema 不符时回退现状（不恢复磁盘内容）。
func TestTaskSnapshotMissingOrCorruptFallsBack(t *testing.T) {
	setupTaskSnapshotDir(t)

	if _, _, ok := buildTaskRestoreFromDisk(); ok {
		t.Fatal("无快照文件时不应恢复")
	}

	dir := historyDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, taskDiskSnapshotName), []byte("不是 JSON"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTaskDiskSnapshot(); err == nil {
		t.Fatal("损坏快照应返回错误")
	}
	if _, _, ok := buildTaskRestoreFromDisk(); ok {
		t.Fatal("损坏快照时不应恢复")
	}

	broken, _ := json.Marshal(taskDiskSnapshot{Schema: "cfdata.unknown"})
	if err := os.WriteFile(filepath.Join(dir, taskDiskSnapshotName), broken, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := buildTaskRestoreFromDisk(); ok {
		t.Fatal("未知 schema 时不应恢复")
	}
}

// 恢复来源优先级：内存回放可用时用内存（fromDisk=false）；内存为空且无任务
// 运行时用磁盘（fromDisk=true）；有任务运行时不读磁盘。
func TestResolveTaskRestorePriority(t *testing.T) {
	setupTaskSnapshotDir(t)
	runSnapshotTask(t, "优先级任务", "2.2.2.2")

	// 内存回放可用（任务刚结束，缓冲未清）：内存优先
	payload, _, _ := resolveTaskRestore()
	if payload.FromDisk {
		t.Error("内存回放可用时不应使用磁盘快照")
	}
	if !payload.HasTask {
		t.Fatal("任务刚结束时内存回放应有内容")
	}

	// 模拟进程重启：全局会话换成空会话，磁盘快照成为唯一来源
	orig := taskSession
	taskSession = newHubSession()
	defer func() { taskSession = orig }()
	payload, _, _ = resolveTaskRestore()
	if !payload.FromDisk || !payload.HasTask {
		t.Errorf("内存为空且无任务运行时应从磁盘恢复: fromDisk=%v hasTask=%v", payload.FromDisk, payload.HasTask)
	}
	if payload.Label != "优先级任务" {
		t.Errorf("磁盘恢复 label = %q, 期望 优先级任务", payload.Label)
	}

	// 任务运行中：不读磁盘快照
	taskSession = orig
	removeLatestHistory(t)
	release := make(chan struct{})
	started := make(chan struct{})
	done := make(chan struct{})
	taskSession.startTaskNamed("运行中任务", "official", nil, func(ctx context.Context, sess *appSession) {
		defer close(done)
		close(started)
		<-release
	})
	<-started
	payload, _, _ = resolveTaskRestore()
	if payload.FromDisk {
		t.Error("任务运行中不应使用磁盘快照")
	}
	close(release)
	<-done
	waitHubIdle(t)
	waitForTaskTail(t)
}
