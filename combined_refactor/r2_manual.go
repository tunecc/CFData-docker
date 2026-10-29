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
	content := formatSubstoreResults(rows)
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
	cfg, err := readScheduleConfigFile(manualR2ConfigPath())
	if err != nil {
		session.sendWSMessage("log", "读取 R2 配置失败，已跳过手动结果上传: "+err.Error())
		return
	}
	if !cfg.R2Enabled {
		session.sendWSMessage("log", "R2 上传未启用，已跳过手动结果上传")
		return
	}
	workerURL := strings.TrimSpace(cfg.R2WorkerURL)
	workerToken := strings.TrimSpace(cfg.R2WorkerToken)
	if workerURL == "" || workerToken == "" {
		session.sendWSMessage("log", "R2 上传已启用，但 Worker 地址或 token 未配置，已跳过手动结果上传")
		return
	}
	filename := strings.TrimSpace(cfg.R2Filename)
	if filename == "" {
		filename = defaultR2Filename
	}
	if err := uploadR2Content(workerURL, workerToken, filename, content); err != nil {
		session.sendWSMessage("log", "R2 上传失败，保留 R2 上的旧内容: "+err.Error())
		return
	}
	session.sendWSMessage("log", fmt.Sprintf("R2 上传完成：%d 条结果已上传到 %s", countSubstoreEntries(content), filename))
}

func filterR2QualifiedRows(rows []cliResultRow, speedMin float64) []cliResultRow {
	return filterCLIResultRowsByQualified(rows, "qualified", speedMin)
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
