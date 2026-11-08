package main

// 磁盘任务快照：任务结束时把全局任务会话的回放内容（运行日志 + 各类结果行）
// 与终态快照原子落盘，进程重启后新连接仍能完整恢复上次任务视图，对齐
// subs-check-pro「文件持久化 + 打开即见」的体验。内存回放可用时磁盘快照
// 不参与，恢复优先级见 resolveTaskRestore。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	taskDiskSnapshotSchema = "cfdata.task-state.v1"
	taskDiskSnapshotName   = "task-state.json"
)

// taskDiskSnapshot 是全局任务会话回放缓冲的完整序列化；各类结果行沿用
// recordReplayLocked 的回放上限，与内存缓冲逐条一致。
type taskDiskSnapshot struct {
	Schema   string                 `json:"schema"`
	Mode     string                 `json:"mode"`
	Label    string                 `json:"label"`
	Snapshot backgroundTaskSnapshot `json:"snapshot"`
	Logs     []string               `json:"logs,omitempty"`
	Scan     []ScanResult           `json:"scan,omitempty"`
	Test     []TestResult           `json:"test,omitempty"`
	Speed    []map[string]string    `json:"speed,omitempty"`
	NSBLive  []nsbScanMessage       `json:"nsbLive,omitempty"`
	DC       []DataCenterInfo       `json:"dc,omitempty"`
	NSBFinal *nsbCSVCompletePayload `json:"nsbFinal,omitempty"`
}

func (snap *taskDiskSnapshot) hasContent() bool {
	return snap.Label != "" || len(snap.Logs) > 0 || len(snap.Scan) > 0 || len(snap.Test) > 0 ||
		len(snap.Speed) > 0 || len(snap.NSBLive) > 0 || len(snap.DC) > 0 || snap.NSBFinal != nil
}

// captureTaskDiskSnapshot 汇总全局任务会话的终态与回放数据；锁序与
// buildTaskRestore 一致（taskMutex→replayMutex），避免与 beginTask 形成锁序倒置。
// 无任何可恢复内容时返回 nil（不覆盖已有快照文件）。
func (s *appSession) captureTaskDiskSnapshot() *taskDiskSnapshot {
	s.taskMutex.Lock()
	snapshot := s.backgroundSnapshot
	// 调用点位于 markTaskFinished 之后、endTask 之前：终态已写入快照，
	// isTaskRunning 仍为 true，这里固定为 false 表达“上次任务已结束”
	snapshot.Running = false
	s.taskMutex.Unlock()
	s.replayMutex.Lock()
	defer s.replayMutex.Unlock()
	logs := append([]string(nil), s.taskLogs...)
	scan := append([]ScanResult(nil), s.scanReplay...)
	test := append([]TestResult(nil), s.testReplay...)
	speed := append([]map[string]string(nil), s.speedReplay...)
	nsbLive := append([]nsbScanMessage(nil), s.nsbLiveReplay...)
	dc := append([]DataCenterInfo(nil), s.dcReplay...)
	var nsbFinal *nsbCSVCompletePayload
	if s.nsbFinalReplay != nil {
		payloadCopy := *s.nsbFinalReplay
		nsbFinal = &payloadCopy
	}
	snap := &taskDiskSnapshot{
		Schema:   taskDiskSnapshotSchema,
		Mode:     snapshot.Mode,
		Label:    snapshot.Label,
		Snapshot: snapshot,
		Logs:     logs,
		Scan:     scan,
		Test:     test,
		Speed:    speed,
		NSBLive:  nsbLive,
		DC:       dc,
		NSBFinal: nsbFinal,
	}
	if !snap.hasContent() {
		return nil
	}
	return snap
}

// persistTaskSnapshot 把最终回放内容原子落盘。调用点位于 markTaskFinished
// 之后、endTask 之前：终态已写入快照，且 isTaskRunning 仍为 true，beginTask
// 无法并发清空回放缓冲。落盘失败仅记日志，不影响任务终态。
func persistTaskSnapshot(s *appSession) {
	snap := s.captureTaskDiskSnapshot()
	if snap == nil {
		return
	}
	data, err := json.Marshal(snap)
	if err != nil {
		fmt.Printf("[taskstate] 任务快照序列化失败: %v\n", err)
		return
	}
	if err := ensureTaskSnapshotDir(); err != nil {
		fmt.Printf("[taskstate] 任务快照目录创建失败: %v\n", err)
		return
	}
	if err := atomicWriteFile(filepath.Join(historyDir(), taskDiskSnapshotName), data, 0644); err != nil {
		fmt.Printf("[taskstate] 任务快照写入失败: %v\n", err)
	}
}

// ensureTaskSnapshotDir 保证快照目录存在；调用方（任务收尾）此前可能从未
// 写过 history 目录（如仅运行过 Web 任务而无历史记录时）。
func ensureTaskSnapshotDir() error {
	return os.MkdirAll(historyDir(), 0755)
}

// loadTaskDiskSnapshot 读取磁盘任务快照；缺失、损坏或 schema 不符时返回错误，
// 由调用方回退现状（仅历史摘要恢复）。
func loadTaskDiskSnapshot() (*taskDiskSnapshot, error) {
	data, err := os.ReadFile(filepath.Join(historyDir(), taskDiskSnapshotName))
	if err != nil {
		return nil, err
	}
	var snap taskDiskSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("任务快照解析失败: %w", err)
	}
	if snap.Schema != taskDiskSnapshotSchema {
		return nil, fmt.Errorf("未知任务快照 schema: %q", snap.Schema)
	}
	return &snap, nil
}

// buildTaskRestoreFromDisk 从磁盘快照重建与内存回放完全同构的恢复载荷：
// 结果行经 appendTaskRestoreChunks 分片，payload 带 fromDisk 标记。快照缺失、
// 损坏或无内容时 ok 为 false。
func buildTaskRestoreFromDisk() (taskRestorePayload, []taskRestoreChunk, bool) {
	snap, err := loadTaskDiskSnapshot()
	if err != nil || snap == nil || !snap.hasContent() {
		return taskRestorePayload{}, nil, false
	}
	payload := taskRestorePayload{
		HasTask:  true,
		Running:  false,
		Label:    snap.Label,
		Mode:     snap.Mode,
		Snapshot: snap.Snapshot,
		Logs:     snap.Logs,
		FromDisk: true,
	}
	chunks := make([]taskRestoreChunk, 0, 8)
	chunks = appendTaskRestoreChunks(chunks, "scan", len(snap.Scan), func(from, to int) interface{} { return snap.Scan[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "test", len(snap.Test), func(from, to int) interface{} { return snap.Test[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "speed", len(snap.Speed), func(from, to int) interface{} { return snap.Speed[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "nsbLive", len(snap.NSBLive), func(from, to int) interface{} { return snap.NSBLive[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "dc", len(snap.DC), func(from, to int) interface{} { return snap.DC[from:to] })
	if snap.NSBFinal != nil {
		chunks = appendTaskRestoreChunks(chunks, "nsbHeaders", len(snap.NSBFinal.Headers), func(from, to int) interface{} { return snap.NSBFinal.Headers[from:to] })
		chunks = appendTaskRestoreChunks(chunks, "nsbRows", len(snap.NSBFinal.Rows), func(from, to int) interface{} { return snap.NSBFinal.Rows[from:to] })
	}
	return payload, chunks, true
}

// resolveTaskRestore 决定新连接的恢复来源：内存回放可用（HasTask）时完全
// 沿用现状；内存为空且无任务运行时从磁盘快照恢复；快照缺失时回退现状。
// lastSeq 始终取内存会话当前序号，后续增量消息按现有 seq 过滤衔接。
func resolveTaskRestore() (taskRestorePayload, []taskRestoreChunk, uint64) {
	payload, chunks, lastSeq := taskSession.buildTaskRestore()
	if !payload.HasTask && !anyTaskRunning() {
		if diskPayload, diskChunks, ok := buildTaskRestoreFromDisk(); ok {
			return diskPayload, diskChunks, lastSeq
		}
	}
	return payload, chunks, lastSeq
}
