package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// 统一任务编排层（学习 subs-check-pro 后的结构收敛，见
// docs/analyses/subs-check-pro-learnings.md 建议⑩）：官方/非标任务各只有
// 一条编排函数，Web 手动路径与调度路径共用，差异由 options 表达——
// 消除"每加一个任务收尾功能要双份接线"的重复。

// officialFlowOptions 描述一次官方优选任务的完整编排参数。
type officialFlowOptions struct {
	ipType      int
	threads     int
	port        int
	delay       int
	scanMode    string
	targetDC    string
	stopCount   int
	libraryIPs  []string
	fromLibrary bool
	// 自动测速（扫描 → 详细测试 → 批量测速）；Web 路径由前端开关决定，
	// 调度路径恒为 true（与原 CLI 管线一致）
	autoSpeed  bool
	speedPort  int
	speedURL   string
	speedLimit int
	speedMin   float64
	// 调度路径：任务结束后按 CLI 语义写导出文件并按配置 R2 上传
	exportAndUpload bool
	cfg             *cliConfig
	// 完成回调：调度路径用于同步结果；Web 路径为 nil
	onDone func(error)
}

func (o officialFlowOptions) done(err error) {
	if o.onDone != nil {
		o.onDone(err)
	}
}

// nsbFlowOptions 描述一次非标优选任务的完整编排参数。
type nsbFlowOptions struct {
	fileName     string
	fileContent  string
	sourceURL    string
	outFile      string
	threads      int
	fallbackPort int
	speedTest    int
	speedURL     string
	enableTLS    bool
	delay        int
	resultLimit  int
	targetDC     string
	speedMin     float64
	speedLimit   int
	compact      bool
	scanMode     string
	relinkHost   string
	// 调度路径：结束后按 CLI 语义写导出文件并按配置 R2 上传
	exportAndUpload bool
	cfg             *cliConfig
	onDone          func(error)
}

func (o nsbFlowOptions) done(err error) {
	if o.onDone != nil {
		o.onDone(err)
	}
}

// runOfficialTaskFlow 编排官方优选任务：扫描（库/默认池）→ 详细测试 →
// 批量测速 → 收尾（调度路径导出+R2；Web 路径取消补传 R2）。
func runOfficialTaskFlow(ctx context.Context, session *appSession, opts officialFlowOptions) error {
	scanMode := opts.scanMode
	if scanMode == "" {
		scanMode = scanModeTCPing
	}
	if opts.fromLibrary {
		runOfficialTaskWithPool(ctx, session, opts.libraryIPs, opts.threads, opts.port, opts.delay, scanMode, opts.targetDC, opts.stopCount, true)
	} else {
		runOfficialTask(ctx, session, opts.ipType, opts.threads, opts.port, opts.delay, scanMode, opts.targetDC, opts.stopCount)
	}
	if ctx.Err() != nil {
		err := ctx.Err()
		opts.done(err)
		return err
	}

	// 自动测速关闭且无需导出时：仅扫描（与原 Web 内联行为一致）
	if !opts.autoSpeed && !opts.exportAndUpload {
		opts.done(nil)
		return nil
	}

	session.scanMutex.Lock()
	scanResults := append([]ScanResult(nil), session.scanResults...)
	session.scanMutex.Unlock()
	if len(scanResults) == 0 && opts.exportAndUpload {
		err := errors.New("官方模式未发现有效 IP")
		opts.done(err)
		return err
	}

	// 调度路径 cfg.dc 可能是小写/带空格：与旧管线一致做规范化
	dc := normalizeOfficialTargetDC(opts.targetDC)
	if dc == "" {
		dc = pickBestDataCenter(scanResults)
		session.sendWSMessage("log", "自动选择数据中心: "+dc)
	}
	if dc == "" {
		if opts.exportAndUpload {
			// 无数据中心：仅输出扫描结果（原 CLI 管线语义）
			err := writeCLIExportAndMaybeUpload(opts.cfg, applyCLIExportFilters(opts.cfg, officialScanRows(scanResults, scanMode), opts.cfg.speedMin), nil, "official-scan")
			opts.done(err)
			return err
		}
		opts.done(nil)
		return nil
	}
	if ctx.Err() != nil {
		err := ctx.Err()
		opts.done(err)
		return err
	}

	// R2 上传统一由收尾分支处理：详细测试一律延后触发，避免双发
	session.testMutex.Lock()
	session.testResults = nil
	session.testMutex.Unlock()
	runDetailedTest(ctx, session, dc, opts.speedPort, opts.delay, scanMode, true)
	if ctx.Err() != nil {
		err := ctx.Err()
		opts.done(err)
		return err
	}

	session.testMutex.Lock()
	results := append([]TestResult(nil), session.testResults...)
	session.testMutex.Unlock()

	if opts.autoSpeed {
		if opts.speedLimit <= 0 {
			session.sendWSMessage("log", "官方测速已关闭（测速结果数量为 0），仅保留详细测试结果")
		} else if len(results) == 0 {
			session.sendWSMessage("log", "没有可用的详细测试结果，跳过测速")
		} else {
			speedURL := strings.TrimSpace(opts.speedURL)
			if speedURL == "" || isAutoSpeedURL(speedURL) {
				speedURL = speedTestURL
			}
			runOfficialSpeedBatch(ctx, session, opts.speedPort, speedURL, opts.speedLimit, opts.speedMin, results, false)
		}
		// 测速在产出任何结果之前被取消时补传延迟筛选结果（仅 Web 路径；
		// 调度路径由导出收尾统一上传，避免重复）
		if ctx.Err() != nil && !opts.exportAndUpload && !officialSpeedAttempted(session) {
			session.testMutex.Lock()
			fallback := append([]TestResult(nil), session.testResults...)
			session.testMutex.Unlock()
			triggerManualR2Upload(session, officialResultRows(nil, fallback, scanMode))
		}
	}

	var err error
	if opts.exportAndUpload {
		session.scanMutex.Lock()
		scanResults = append([]ScanResult(nil), session.scanResults...)
		session.scanMutex.Unlock()
		session.testMutex.Lock()
		results = append([]TestResult(nil), session.testResults...)
		session.testMutex.Unlock()
		allRows := officialResultRows(scanResults, results, scanMode)
		var r2Rows []cliResultRow
		if len(results) > 0 {
			r2Rows = scheduledR2Rows(allRows, opts.speedLimit > 0, opts.speedMin)
		}
		err = writeCLIExportAndMaybeUpload(opts.cfg, applyCLIExportFilters(opts.cfg, allRows, opts.speedMin), r2Rows, "official")
	}
	opts.done(err)
	return err
}

// runNSBTaskFlow 编排非标优选任务：输入解析（文件/URL/现成内容）→
// runNSBTask → 收尾（调度路径导出+R2）。
func runNSBTaskFlow(ctx context.Context, session *appSession, opts nsbFlowOptions) error {
	fileName := opts.fileName
	fileContent := opts.fileContent
	switch {
	case strings.TrimSpace(fileContent) != "":
		// Web 上传的现成内容，直接使用
	case strings.TrimSpace(fileName) != "":
		// 文件路径优先于 URL（对齐旧 loadNSBInput：file 分支在前）
		content, err := getFileContent(fileName)
		if err != nil {
			opts.done(err)
			return err
		}
		fileContent = content
	case strings.TrimSpace(opts.sourceURL) != "":
		session.sendWSMessage("log", "正在获取非标网络输入: "+opts.sourceURL)
		content, err := getURLContentViaProxy(ctx, opts.sourceURL)
		if err != nil {
			if ctx.Err() != nil {
				err = ctx.Err()
			} else {
				err = fmt.Errorf("获取非标网络输入失败: %w", err)
				session.sendWSMessage("error", err.Error())
			}
			opts.done(err)
			return err
		}
		fileName = opts.sourceURL
		fileContent = content
	default:
		err := errors.New("非标模式需要提供 IP 文件或网络 URL")
		opts.done(err)
		return err
	}

	scanMode := opts.scanMode
	if scanMode == "" {
		scanMode = scanModeTCPing
	}
	relinkHost := opts.relinkHost
	runNSBTask(ctx, session, fileName, fileContent, opts.outFile, opts.threads, opts.fallbackPort, opts.speedTest, opts.speedURL, opts.enableTLS, opts.delay, opts.resultLimit, opts.targetDC, opts.speedMin, opts.speedLimit, opts.compact, scanMode, relinkHost)
	if ctx.Err() != nil {
		err := ctx.Err()
		opts.done(err)
		return err
	}

	var err error
	if opts.exportAndUpload {
		session.nsbMutex.Lock()
		rows := nsbPayloadRows(session.nsbHeaders, session.nsbRows)
		session.nsbMutex.Unlock()
		speedRan := opts.speedTest > 0 && opts.speedLimit > 0
		r2Rows := scheduledR2Rows(rows, speedRan, opts.speedMin)
		rows = applyCLIExportFilters(opts.cfg, rows, opts.speedMin)
		if len(rows) == 0 && len(r2Rows) == 0 {
			session.sendWSMessage("log", "没有符合导出条件的结果")
		} else {
			err = writeCLIExportAndMaybeUpload(opts.cfg, rows, r2Rows, "nsb")
		}
	}
	opts.done(err)
	return err
}
