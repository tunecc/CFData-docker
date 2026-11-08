package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var globalRunningTasks int32
var globalTaskMutex sync.Mutex

// taskCancels 登记所有活跃任务的取消函数，供优雅停机统一取消（含 CLI/调度会话）。
var taskCancels sync.Map

// 回放缓冲上限：全局任务会话在内存中为“当前/上一次任务”保留的各类数据条数。
const (
	taskLogReplayLimit  = 500
	scanReplayLimit     = 20000
	testReplayLimit     = 20000
	speedReplayLimit    = 2000
	nsbLiveReplayLimit  = 20000
	taskRestoreChunkLen = 400
)

// taskSession 是进程级全局任务会话：Web 触发的任务在其上执行，与任何
// WebSocket 连接的生命周期解耦——关闭页面不会中断任务，新连接建立后可
// 从回放缓冲恢复任务状态。
var taskSession = newHubSession()

func anyTaskRunning() bool {
	return atomic.LoadInt32(&globalRunningTasks) > 0
}

// newHubSession 创建全局任务会话：任务消息经 broadcast 广播给所有在线连接。
func newHubSession() *appSession {
	s := &appSession{progressState: map[string][2]int{}}
	s.broadcast = s.emitTaskMessage
	return s
}

func safeGo(label string, session *appSession, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("%s panic: %v", label, r)
				fmt.Printf("%s\n%s\n", msg, debug.Stack())
				recordProgramDebugError("panic", fmt.Sprintf("%s\n%s", msg, debug.Stack()))
				if session != nil {
					session.sendWSMessage("error", "内部错误: "+msg)
				}
			}
		}()
		fn()
	}()
}

func (s *appSession) sendWSMessage(msgType string, data interface{}) {
	if msgType == "log" && s.emit == nil {
		if text, ok := data.(string); ok {
			for _, out := range s.logThrottle.pass(text) {
				s.sendLogLine(out)
			}
			return
		}
	}
	s.dispatchOutgoing(msgType, data)
}

// sendLogLine 发送单条日志并同步任务快照，供日志合并节流复用。
func (s *appSession) sendLogLine(text string) {
	s.dispatchOutgoing("log", text)
}

// dispatchOutgoing 按会话类型输出消息：CLI 会话交给 emit；全局任务会话
// 记录回放并广播；普通连接会话直写自己的 WebSocket。
func (s *appSession) dispatchOutgoing(msgType string, data interface{}) {
	if s.emit != nil && s.broadcast == nil {
		if msgType == "error" || msgType == "github_upload_error" {
			recordProgramDebugError(msgType, data)
		} else if msgType == "log" || msgType == "github_upload_status" || msgType == "version_info" {
			recordDebugNotice(msgType, data)
		}
		s.emit(msgType, data)
		return
	}
	s.updateBackgroundSnapshotFromMessage(msgType, data)
	if s.broadcast != nil {
		s.broadcast(msgType, data)
		return
	}
	if s.ws == nil {
		return
	}
	s.sendWSMessageDirect(msgType, data)
}

// setUploadNote 记录本会话最近一次 R2 自动上传的结果说明，用于上次运行摘要。
func (s *appSession) setUploadNote(note string) {
	s.uploadNoteMutex.Lock()
	s.uploadNote = note
	s.uploadNoteMutex.Unlock()
}

func (s *appSession) uploadNoteValue() string {
	s.uploadNoteMutex.Lock()
	defer s.uploadNoteMutex.Unlock()
	return s.uploadNote
}

// finalResultCount 返回任务结束时 session 内的最终结果行数。
func (s *appSession) finalResultCount() int {
	s.testMutex.Lock()
	testCount := len(s.testResults)
	s.testMutex.Unlock()
	if testCount > 0 {
		return testCount
	}
	s.scanMutex.Lock()
	scanCount := len(s.scanResults)
	s.scanMutex.Unlock()
	if scanCount > 0 {
		return scanCount
	}
	s.nsbMutex.Lock()
	nsbCount := len(s.nsbRows)
	s.nsbMutex.Unlock()
	return nsbCount
}

func (s *appSession) sendWSMessageDirect(msgType string, data interface{}) {
	if s.client == nil {
		return
	}
	msg := map[string]interface{}{
		"type": msgType,
		"data": data,
	}
	if msgType == "error" || msgType == "github_upload_error" {
		recordProgramDebugError(msgType, data)
	} else if msgType == "log" || msgType == "github_upload_status" || msgType == "version_info" {
		recordDebugNotice(msgType, data)
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		recordProgramDebugError("websocket_marshal", err.Error())
		return
	}
	if !s.client.writePayload(payload) {
		recordProgramDebugError("websocket_write", "connection closed")
	}
}

func (s *appSession) startTask(run taskStarter) {
	s.startTaskNamed("任务", "", nil, run)
}

// startTaskNamed 在当前会话上启动任务。在全局任务会话上启动的任务独立于
// 任何 WebSocket 连接运行，关闭页面不会中断。
// startTaskNamed 在当前会话上启动任务。在全局任务会话上启动的任务独立于
// 任何 WebSocket 连接运行，关闭页面不会中断。返回是否成功启动（调度路径
// 据此判断防重入跳过；既有调用方忽略返回值不受影响）。
func (s *appSession) startTaskNamed(label string, mode string, params map[string]interface{}, run taskStarter) bool {
	ctx, cancel := context.WithCancel(context.Background())
	started := s.beginTask(cancel, label, mode, params)
	if !started {
		cancel()
		s.sendWSMessage("error", "已有任务正在运行，请等待完成后再试")
		return false
	}
	s.setScanFailureSummary("")
	s.setFlowFailure("")
	safeGo("task", s, func() {
		defer cancel()
		defer func() {
			// 先捕获调度标志：endTask 会复位 scheduleRun，而失败分支在其之后
			scheduled := s.scheduleRun
			snapshot := s.backgroundSummary()
			// 先把终态写入快照再结束任务计数，保证恢复视图能看到最终阶段
			s.markTaskFinished(ctx.Err() != nil)
			s.endTask()
			// 空标签的同步任务（如调度流水线内部步骤）不覆盖上次运行摘要
			if label != "" {
				startedAt := snapshot.StartedAt
				if startedAt.IsZero() {
					startedAt = time.Now()
				}
				success := ctx.Err() == nil
				// 管线错误（非用户停止）把任务归类为失败：调度路径发失败通知
				// 并写失败历史；Web 路径错误仍走 WS error，不发失败通知
				failureText := ""
				if success {
					failureText = s.flowFailureValue()
					success = failureText == ""
				}
				recordLastRun(mode, label, startedAt, success, s.finalResultCount(), s.uploadNoteValue())
				s.sendWSMessage("last_run", lastRunPayload())
				// 扫描/测速主任务完成时发送 webhook 通知并落盘历史记录；
				// 被用户停止的不通知、不写历史
				if success && (mode == "official" || mode == "nsb") {
					sendTaskNotifyAsync("complete", mode, label, s.finalResultCount(), startedAt, "")
					persistTaskHistory(s, mode, label, startedAt, "")
				} else if !success && failureText != "" && scheduled {
					sendTaskNotifyAsync("failure", mode, label, s.finalResultCount(), startedAt, failureText)
					persistTaskHistory(s, mode, label, startedAt, failureText)
				}
			}
			if ctx.Err() != nil {
				s.sendWSMessage("task_stopped", nil)
				return
			}
			s.sendWSMessage("task_complete", nil)
		}()
		run(ctx, s)
	})
	return true
}

func (s *appSession) stopTask() {
	s.cancelTask(true)
}

func (s *appSession) cancelTaskSilently() {
	s.cancelTask(false)
}

func (s *appSession) cancelTask(withLog bool) {
	s.taskMutex.Lock()
	cancel := s.taskCancel
	s.taskMutex.Unlock()
	if cancel != nil {
		cancel()
		if withLog {
			s.sendWSMessage("log", "已发送强制终止信号，正在清理当前任务...")
		}
	}
}

// markTaskFinished 把任务终态（完成/已停止）写入快照，供新连接恢复视图使用。
func (s *appSession) markTaskFinished(stopped bool) {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if !s.isTaskRunning {
		return
	}
	if stopped {
		s.backgroundSnapshot.Phase = "已停止"
		s.backgroundSnapshot.Message = "任务已停止"
	} else {
		s.backgroundSnapshot.Phase = "完成"
		s.backgroundSnapshot.Message = "任务完成"
	}
	s.backgroundSnapshot.Running = false
	s.backgroundSnapshot.UpdatedAt = time.Now()
}

func (s *appSession) beginTask(cancel context.CancelFunc, label string, mode string, params map[string]interface{}) bool {
	globalTaskMutex.Lock()
	defer globalTaskMutex.Unlock()
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if anyTaskRunning() {
		return false
	}
	if s.isTaskRunning {
		return false
	}
	s.isTaskRunning = true
	s.taskCancel = cancel
	now := time.Now()
	s.backgroundSnapshot = backgroundTaskSnapshot{Label: label, Mode: mode, Phase: "准备中", Message: "任务准备中", Running: true, StartedAt: now, UpdatedAt: now, Params: params}
	if s.broadcast != nil {
		// 全局任务会话在新任务开始时清空上一任务的回放数据
		s.resetReplayState()
	}
	atomic.AddInt32(&globalRunningTasks, 1)
	taskCancels.Store(s, cancel)
	return true
}

// cancelAllTasks 取消所有活跃任务（优雅停机时调用）。
func cancelAllTasks() {
	taskCancels.Range(func(_, value any) bool {
		if cancel, ok := value.(context.CancelFunc); ok {
			cancel()
		}
		return true
	})
}

func (s *appSession) endTask() {
	globalTaskMutex.Lock()
	defer globalTaskMutex.Unlock()
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if s.isTaskRunning {
		atomic.AddInt32(&globalRunningTasks, -1)
	}
	taskCancels.Delete(s)
	s.scheduleRun = false
	s.isTaskRunning = false
	s.taskCancel = nil
	s.backgroundSnapshot.Running = false
	s.backgroundSnapshot.UpdatedAt = time.Now()
}

// emitTaskMessage 为全局任务消息分配单调递增序号、写入回放缓冲，再广播给
// 所有在线连接。序号让前端能区分“已包含在恢复载荷里的旧消息”与“增量消息”，
// 恢复与实时消息交错时也不会出现重复行或缺行。
func (s *appSession) emitTaskMessage(msgType string, data interface{}) {
	s.replayMutex.Lock()
	s.recordReplayLocked(msgType, data)
	s.msgSeq++
	seq := s.msgSeq
	s.replayMutex.Unlock()
	broadcastTaskEnvelope(seq, msgType, data)
}

func appendCapped[T any](slice []T, value T, limit int) []T {
	slice = append(slice, value)
	if len(slice) > limit {
		slice = slice[len(slice)-limit:]
	}
	return slice
}

// recordReplayLocked 把任务消息写入回放缓冲；调用方需持有 replayMutex。
func (s *appSession) recordReplayLocked(msgType string, data interface{}) {
	switch msgType {
	case "log":
		if text, ok := data.(string); ok {
			s.taskLogs = appendCapped(s.taskLogs, text, taskLogReplayLimit)
		}
	case "scan_result":
		if res, ok := data.(ScanResult); ok {
			s.scanReplay = appendCapped(s.scanReplay, res, scanReplayLimit)
		}
	case "test_result":
		if res, ok := data.(TestResult); ok {
			s.testReplay = appendCapped(s.testReplay, res, testReplayLimit)
		}
	case "speed_test_result":
		if res, ok := data.(map[string]string); ok {
			s.speedReplay = appendCapped(s.speedReplay, res, speedReplayLimit)
		}
	case "nsb_scan_result":
		if res, ok := data.(nsbScanMessage); ok {
			s.nsbLiveReplay = appendCapped(s.nsbLiveReplay, res, nsbLiveReplayLimit)
		}
	case "nsb_scan_sorted":
		if rows, ok := data.([]nsbScanMessage); ok {
			if len(rows) > nsbLiveReplayLimit {
				rows = rows[len(rows)-nsbLiveReplayLimit:]
			}
			s.nsbLiveReplay = rows
		}
	case "scan_complete_wait_dc":
		if list, ok := data.([]DataCenterInfo); ok {
			s.dcReplay = list
		}
	case "nsb_csv_complete":
		if payload, ok := data.(nsbCSVCompletePayload); ok {
			payloadCopy := payload
			s.nsbFinalReplay = &payloadCopy
		}
	}
}

// resetReplayState 清空全局任务会话的回放缓冲与上一任务遗留结果，在新任务开始时调用。
func (s *appSession) resetReplayState() {
	s.replayMutex.Lock()
	s.taskLogs = nil
	s.scanReplay = nil
	s.testReplay = nil
	s.speedReplay = nil
	s.nsbLiveReplay = nil
	s.dcReplay = nil
	s.nsbFinalReplay = nil
	s.replayMutex.Unlock()
	s.scanMutex.Lock()
	s.scanResults = nil
	s.scanMutex.Unlock()
	s.testMutex.Lock()
	s.testResults = nil
	s.testMutex.Unlock()
	s.nsbMutex.Lock()
	s.nsbHeaders = nil
	s.nsbRows = nil
	s.nsbCompletePayload = nil
	s.nsbMutex.Unlock()
	s.progressMutex.Lock()
	s.progressState = map[string][2]int{}
	s.progressPrintTime = map[string]time.Time{}
	s.progressPrintPercent = map[string]float64{}
	s.progressMutex.Unlock()
	s.logThrottle = logThrottle{}
	// 任务级临时状态复位：上一任务的 R2 上传备注不能串入下一任务的 last_run。
	// 注意 r2AutoUpload 不在此复位——它由启动处理器在 startTaskNamed 之前写入，
	// 在这里清空会把刚设置的启动参数清掉（顺序：handler 赋值 → beginTask）。
	s.setUploadNote("")
}

// taskRestorePayload 是新连接恢复视图时下发的任务状态与元信息。
type taskRestorePayload struct {
	HasTask  bool                   `json:"hasTask"`
	Running  bool                   `json:"running"`
	Label    string                 `json:"label"`
	Mode     string                 `json:"mode"`
	Snapshot backgroundTaskSnapshot `json:"snapshot"`
	Logs     []string               `json:"logs"`
}

// taskRestoreChunk 携带一类回放数据的一批行，避免单条消息过大。
type taskRestoreChunk struct {
	Kind string      `json:"kind"`
	Rows interface{} `json:"rows"`
}

// buildTaskRestore 汇总全局任务会话的当前状态与回放数据，供新连接恢复页面。
// 快照先经 taskMutex 读取，再在 replayMutex 内拷贝缓冲并读取序号——避免与
// beginTask（taskMutex→replayMutex）形成锁序倒置；恢复载荷与增量消息不重不漏
// 的不变量只依赖缓冲拷贝与序号读取同锁。
func (s *appSession) buildTaskRestore() (taskRestorePayload, []taskRestoreChunk, uint64) {
	snapshot := s.backgroundSummary()
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
	lastSeq := s.msgSeq

	payload := taskRestorePayload{
		HasTask:  snapshot.Label != "" || len(scan) > 0 || len(test) > 0 || len(speed) > 0 || len(nsbLive) > 0 || nsbFinal != nil,
		Running:  snapshot.Running,
		Label:    snapshot.Label,
		Mode:     snapshot.Mode,
		Snapshot: snapshot,
		Logs:     logs,
	}
	chunks := make([]taskRestoreChunk, 0, 8)
	chunks = appendTaskRestoreChunks(chunks, "scan", len(scan), func(from, to int) interface{} { return scan[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "test", len(test), func(from, to int) interface{} { return test[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "speed", len(speed), func(from, to int) interface{} { return speed[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "nsbLive", len(nsbLive), func(from, to int) interface{} { return nsbLive[from:to] })
	chunks = appendTaskRestoreChunks(chunks, "dc", len(dc), func(from, to int) interface{} { return dc[from:to] })
	if nsbFinal != nil {
		chunks = appendTaskRestoreChunks(chunks, "nsbHeaders", len(nsbFinal.Headers), func(from, to int) interface{} { return nsbFinal.Headers[from:to] })
		chunks = appendTaskRestoreChunks(chunks, "nsbRows", len(nsbFinal.Rows), func(from, to int) interface{} { return nsbFinal.Rows[from:to] })
	}
	return payload, chunks, lastSeq
}

func appendTaskRestoreChunks(chunks []taskRestoreChunk, kind string, total int, slice func(from, to int) interface{}) []taskRestoreChunk {
	if total == 0 {
		return chunks
	}
	for from := 0; from < total; from += taskRestoreChunkLen {
		to := from + taskRestoreChunkLen
		if to > total {
			to = total
		}
		chunks = append(chunks, taskRestoreChunk{Kind: kind, Rows: slice(from, to)})
	}
	return chunks
}

func (s *appSession) backgroundSummary() backgroundTaskSnapshot {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	s.backgroundSnapshot.Running = s.isTaskRunning
	return s.backgroundSnapshot
}

func (s *appSession) updateBackgroundSnapshotFromMessage(msgType string, data interface{}) {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if !s.isTaskRunning {
		return
	}
	now := time.Now()
	s.backgroundSnapshot.UpdatedAt = now
	s.backgroundSnapshot.Running = s.isTaskRunning
	s.applyBackgroundMessageLocked(msgType, data)
}

func (s *appSession) applyBackgroundMessageLocked(msgType string, data interface{}) {
	switch msgType {
	case "log":
		s.backgroundSnapshot.Message = fmt.Sprint(data)
	case "scan_progress", "test_progress":
		m, ok := data.(map[string]interface{})
		if !ok {
			return
		}
		phase := "扫描中"
		current := asInt(m["current"])
		total := asInt(m["total"])
		if msgType == "test_progress" {
			phase = "详细测试中"
		}
		s.backgroundSnapshot.ScanFailed = max(current-s.backgroundSnapshot.ScanSuccess, 0)
		s.updateProgressSnapshotLocked(phase, current, total, phase)
	case "official_speed_progress":
		m, ok := data.(map[string]interface{})
		if !ok {
			return
		}
		s.backgroundSnapshot.SpeedTotal = asInt(m["limit"])
		s.backgroundSnapshot.SpeedQualified = asInt(m["qualified"])
		s.updateProgressSnapshotLocked("测速中", asInt(m["qualified"]), asInt(m["limit"]), "测速中")
	case "nsb_progress":
		m, ok := data.(map[string]interface{})
		if !ok {
			return
		}
		text := fmt.Sprint(m["text"])
		if text == "" || text == "<nil>" {
			text = "处理中"
		}
		phase := fmt.Sprint(m["phase"])
		current := asInt(m["current"])
		total := asInt(m["total"])

		if phase == "scan" {
			s.backgroundSnapshot.ScanFailed = max(current-s.backgroundSnapshot.ScanSuccess, 0)
		} else if phase == "speed" {
			s.backgroundSnapshot.SpeedQualified = current
		}
		s.updateProgressSnapshotLocked(text, current, total, text)
	case "scan_result":
		s.backgroundSnapshot.ResultCount++
		s.backgroundSnapshot.ScanSuccess++
		s.backgroundSnapshot.ScanTotal++
	case "nsb_scan_result":
		if s.backgroundSnapshot.Phase == "测速中" {
			s.backgroundSnapshot.SpeedCount++
			s.updateBackgroundSpeedStatsLocked(data)
		} else {
			s.backgroundSnapshot.ResultCount++
			s.backgroundSnapshot.ScanSuccess++
			s.backgroundSnapshot.ScanTotal++
		}
	case "speed_test_result":
		s.backgroundSnapshot.SpeedCount++
		s.updateBackgroundSpeedStatsLocked(data)
	case "scan_complete_wait_dc":
		if list, ok := data.([]DataCenterInfo); ok {
			s.backgroundSnapshot.DCCount = len(list)
			names := make([]string, 0, len(list))
			for _, dc := range list {
				name := dc.DataCenter
				if dc.City != "" {
					name += "(" + dc.City + ")"
				}
				names = append(names, name)
			}
			s.backgroundSnapshot.DCInfo = strings.Join(names, ", ")
		}
		s.backgroundSnapshot.Phase = "扫描完成"
		s.backgroundSnapshot.Message = "扫描完成，等待详细测试或后续阶段"
	case "test_complete":
		s.backgroundSnapshot.Phase = "详细测试完成"
		s.backgroundSnapshot.Message = "详细测试完成"
	case "official_speed_complete", "nsb_speed_complete":
		s.backgroundSnapshot.Phase = "测速完成"
		s.backgroundSnapshot.Message = "测速完成"
		s.finishBackgroundStatsLocked()
	case "nsb_csv_complete":
		s.backgroundSnapshot.Phase = "完成"
		s.backgroundSnapshot.Message = "结果文件已生成"
		s.finishBackgroundStatsLocked()
		if payload, ok := data.(nsbCSVCompletePayload); ok {
			s.nsbMutex.Lock()
			s.nsbCompletePayload = &payload
			s.nsbMutex.Unlock()
		}
	}
}

func (s *appSession) updateBackgroundSpeedStatsLocked(data interface{}) {
	s.backgroundSnapshot.SpeedSuccess++
	s.backgroundSnapshot.SpeedTotal++
	speedText := ""
	switch value := data.(type) {
	case map[string]string:
		speedText = value["speed"]
	case map[string]interface{}:
		speedText = fmt.Sprint(value["speed"])
	case nsbScanMessage:
		speedText = value.Speed
		if value.SpeedQualified {
			s.backgroundSnapshot.SpeedQualified++
			return
		}
	case *nsbScanMessage:
		if value != nil {
			speedText = value.Speed
			if value.SpeedQualified {
				s.backgroundSnapshot.SpeedQualified++
				return
			}
		}
	}
	if speedMB, ok := parseSpeedMBForSort(speedText); ok {
		if speedMB >= s.backgroundSpeedMinLocked() {
			s.backgroundSnapshot.SpeedQualified++
		}
		return
	}
	s.backgroundSnapshot.SpeedSuccess--
	s.backgroundSnapshot.SpeedFailed++
}

func (s *appSession) backgroundSpeedMinLocked() float64 {
	value, ok := s.backgroundSnapshot.Params["speedMin"]
	if !ok {
		return 0
	}
	switch n := value.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		parsed, ok := parseSpeedMBForSort(n + "MB/s")
		if ok {
			return parsed
		}
	}
	return 0
}

func (s *appSession) finishBackgroundStatsLocked() {
}

func (s *appSession) updateProgressSnapshotLocked(phase string, current int, total int, message string) {
	s.backgroundSnapshot.Phase = phase
	s.backgroundSnapshot.Message = message
	s.backgroundSnapshot.Current = current
	s.backgroundSnapshot.Total = total
	if total > 0 {
		s.backgroundSnapshot.Percent = float64(current) / float64(total) * 100
	}
}
