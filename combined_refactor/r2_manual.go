package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

var (
	manualR2UploadMutex sync.Mutex
	manualR2ConfigPath  = defaultCLIConfigPath
)

// 手动任务的 R2 上传只由 Web 会话触发；CLI/调度会话由 writeCLIExportAndMaybeUpload 负责
func triggerManualR2Upload(session *appSession, rows []cliResultRow) {
	if session == nil || session.emit != nil || !session.r2AutoUpload || len(rows) == 0 {
		return
	}
	// 别名模板取自当前配置；读取失败时按默认别名生成，后续上传阶段会记录读取错误并跳过
	aliasTemplate := ""
	if cfg, err := readScheduleConfigFile(manualR2ConfigPath()); err == nil {
		aliasTemplate = cfg.R2AliasFormat
	}
	content := formatSubstoreResultsWithAlias(rows, aliasTemplate)
	if strings.TrimSpace(content) == "" {
		return
	}
	safeGo("r2-upload", session, func() {
		manualR2UploadMutex.Lock()
		defer manualR2UploadMutex.Unlock()
		uploadManualRowsToR2(session, content)
	})
}

func uploadManualRowsToR2(session *appSession, content string) {
	defer updateSessionUploadNote(session)
	cfg, err := readScheduleConfigFile(manualR2ConfigPath())
	if err != nil {
		note := "R2 上传跳过：读取配置失败"
		session.setUploadNote(note)
		session.sendWSMessage("log", "读取 R2 配置失败，已跳过手动结果上传: "+err.Error())
		return
	}
	if !cfg.R2Enabled {
		note := "R2 上传未启用"
		session.setUploadNote(note)
		session.sendWSMessage("log", "R2 上传未启用，已跳过手动结果上传")
		return
	}
	workerURL := strings.TrimSpace(cfg.R2WorkerURL)
	workerToken := strings.TrimSpace(cfg.R2WorkerToken)
	if workerURL == "" || workerToken == "" {
		note := "R2 上传跳过：Worker 地址或 token 未配置"
		session.setUploadNote(note)
		session.sendWSMessage("log", "R2 上传已启用，但 Worker 地址或 token 未配置，已跳过手动结果上传")
		return
	}
	filename := strings.TrimSpace(cfg.R2Filename)
	if filename == "" {
		filename = defaultR2Filename
	}
	session.setUploadNote("R2 上传中…")
	if err := uploadR2Content(workerURL, workerToken, filename, content); err != nil {
		note := "R2 上传失败"
		session.setUploadNote(note)
		session.sendWSMessage("log", "R2 上传失败，保留 R2 上的旧内容: "+err.Error())
		return
	}
	session.setUploadNote(fmt.Sprintf("R2 已上传 %d 条", countSubstoreEntries(content)))
	session.sendWSMessage("log", fmt.Sprintf("R2 上传完成：%d 条结果已上传到 %s", countSubstoreEntries(content), filename))
}

// uploadPanelContentToR2 处理结果操作面板「上传 R2」按钮：
// 内容由前端按导出设置生成，文件名沿用已保存的 R2 文件名，与自动上传规则互不影响。
func uploadPanelContentToR2(session *appSession, content string) {
	if session == nil {
		return
	}
	content = strings.TrimSpace(content)
	if content == "" {
		session.sendWSMessage("r2_upload_error", map[string]interface{}{"message": "没有可上传的结果，请先运行任务或调整筛选"})
		return
	}
	cfg, err := readScheduleConfigFile(manualR2ConfigPath())
	if err != nil {
		session.sendWSMessage("r2_upload_error", map[string]interface{}{"message": "读取 R2 配置失败: " + err.Error()})
		return
	}
	if !cfg.R2Enabled {
		session.sendWSMessage("r2_upload_error", map[string]interface{}{"message": "R2 上传未启用，请先在「R2 上传」分组开启并保存"})
		return
	}
	if strings.TrimSpace(cfg.R2WorkerURL) == "" || strings.TrimSpace(cfg.R2WorkerToken) == "" {
		session.sendWSMessage("r2_upload_error", map[string]interface{}{"message": "R2 上传已启用，但 Worker 地址或 token 未配置，请填写并保存后重试"})
		return
	}
	filename := strings.TrimSpace(cfg.R2Filename)
	if filename == "" {
		filename = defaultR2Filename
	}
	session.sendWSMessage("r2_upload_status", map[string]interface{}{"message": "R2 上传中…"})
	if err := uploadR2Content(cfg.R2WorkerURL, cfg.R2WorkerToken, filename, content); err != nil {
		session.sendWSMessage("r2_upload_error", map[string]interface{}{"message": "R2 上传失败，保留 R2 上的旧内容: " + err.Error()})
		return
	}
	session.sendWSMessage("r2_upload_result", map[string]interface{}{
		"message":  fmt.Sprintf("R2 上传成功：%d 行已写入 %s", countSubstoreEntries(content), filename),
		"filename": filename,
	})
}

// updateSessionUploadNote 把会话内最新的上传说明同步到全局上次运行摘要并推送给页面，
// 使任务结束后才完成的异步上传也能反映在摘要行中。
func updateSessionUploadNote(session *appSession) {
	if session == nil || session.emit != nil {
		return
	}
	updateLastRunUploadNote(session.uploadNoteValue())
	session.sendWSMessage("last_run", lastRunPayload())
}

func countSubstoreEntries(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func nsbSpeedResultRows(rows []nsbScanMessage, results []iptestResult) []cliResultRow {
	speeds := make(map[string]string, len(results))
	for i := range results {
		speed := results[i].speedText
		if speed == "" && results[i].speedTested {
			speed = fmt.Sprintf("%.2fMB/s", results[i].downloadSpeed/1024)
		}
		if !results[i].speedTested && speed == "" {
			continue
		}
		speeds[results[i].ipAddr+":"+strconv.Itoa(results[i].port)] = speed
	}
	out := make([]cliResultRow, 0, len(rows))
	for _, row := range rows {
		ip := strings.TrimSpace(row.IP)
		port := strings.TrimSpace(row.Port)
		speed := row.Speed
		if updated, ok := speeds[ip+":"+port]; ok {
			speed = updated
		}
		out = append(out, cliResultRow{
			"ip":            ip,
			"port":          port,
			"ipport":        ip + ":" + port,
			"tls":           row.TLS,
			"dc":            row.DC,
			"loc":           row.Loc,
			"region":        row.Region,
			"city":          row.City,
			"latency":       row.Latency,
			"lossRate":      row.LossRate,
			"speed":         speed,
			"outboundIP":    row.OutboundIP,
			"ipType":        row.IPType,
			"asnNumber":     row.ASNNumber,
			"asnOrg":        row.ASNOrg,
			"originalInput": row.OriginalInput,
		})
	}
	return out
}
