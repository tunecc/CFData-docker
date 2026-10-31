package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/robfig/cron/v3"
)

type scheduleMode string

const (
	scheduleDisabled scheduleMode = "disabled"
	scheduleCron     scheduleMode = "cron"
	scheduleInterval scheduleMode = "interval"
)

const (
	defaultScheduleInterval = 2880 * time.Minute
	minScheduleInterval     = 60 * time.Minute
)

type schedulePlan struct {
	Enabled        bool
	Mode           scheduleMode
	CronExpression string
	CronSchedule   cron.Schedule
	Interval       time.Duration
	Warning        string
}

// describeSchedulePlan 返回调度计划的中文摘要（原 schedule_menu.go，CLI 菜单删除后保留）
func describeSchedulePlan(plan schedulePlan) string {
	if !plan.Enabled {
		return "已关闭"
	}
	desc := "已启用（" + plan.String() + "）"
	if plan.Warning != "" {
		desc += "；" + plan.Warning
	}
	return desc
}

func normalizeSchedulePlan(cfg cliFileConfig) schedulePlan {
	plan := schedulePlan{Enabled: cfg.ScheduleEnabled}
	if !plan.Enabled {
		plan.Mode = scheduleDisabled
		return plan
	}

	if expr := strings.TrimSpace(cfg.CronExpression); expr != "" {
		schedule, err := cron.ParseStandard(expr)
		if err == nil {
			plan.Mode = scheduleCron
			plan.CronExpression = expr
			plan.CronSchedule = schedule
			return plan
		}
		plan.Warning = fmt.Sprintf("Cron 表达式 %q 无效，回退到检测间隔: %v", expr, err)
	}

	plan.Mode = scheduleInterval
	plan.Interval = normalizeScheduleInterval(cfg.CheckInterval)
	return plan
}

func normalizeScheduleInterval(minutes int) time.Duration {
	if minutes <= 0 {
		return defaultScheduleInterval
	}
	interval := time.Duration(minutes) * time.Minute
	if interval < minScheduleInterval {
		return minScheduleInterval
	}
	return interval
}

func (p schedulePlan) String() string {
	switch p.Mode {
	case scheduleCron:
		return fmt.Sprintf("Cron %q", p.CronExpression)
	case scheduleInterval:
		return fmt.Sprintf("每 %d 分钟", int(p.Interval/time.Minute))
	default:
		return "未启用"
	}
}

const defaultSchedulerPollInterval = 10 * time.Second

type schedulerHooks struct {
	now              func() time.Time
	newTimer         func(time.Duration) (<-chan time.Time, func())
	newTicker        func(time.Duration) (<-chan time.Time, func())
	pollInterval     time.Duration
	intervalOverride time.Duration
	busy             func() bool
	run              func(trigger string) string
}

func defaultSchedulerHooks() schedulerHooks {
	return schedulerHooks{
		now: time.Now,
		newTimer: func(d time.Duration) (<-chan time.Time, func()) {
			timer := time.NewTimer(d)
			return timer.C, func() { timer.Stop() }
		},
		newTicker: func(d time.Duration) (<-chan time.Time, func()) {
			ticker := time.NewTicker(d)
			return ticker.C, func() { ticker.Stop() }
		},
		pollInterval: defaultSchedulerPollInterval,
	}
}

type schedulerStatus struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	CronExpression  string `json:"cronExpression,omitempty"`
	IntervalMinutes int    `json:"intervalMinutes,omitempty"`
	Warning         string `json:"warning,omitempty"`
	NextRun         string `json:"nextRun,omitempty"`
	LastRun         string `json:"lastRun,omitempty"`
	LastResult      string `json:"lastResult,omitempty"`
	Running         bool   `json:"running"`
}

type scanScheduler struct {
	configPath string
	hooks      schedulerHooks

	mu         sync.Mutex
	plan       schedulePlan
	nextRun    time.Time
	lastRun    time.Time
	lastResult string
	lastMtime  time.Time
	mtimeKnown bool

	lastReloadErr   string
	lastReloadErrAt time.Time
	started         bool
	stopped         bool
	stopCh          chan struct{}
	reloadCh        chan struct{}
	loopDone        chan struct{}

	running atomic.Bool
	wg      sync.WaitGroup
}

func newScanScheduler(configPath string) *scanScheduler {
	return &scanScheduler{configPath: configPath, hooks: defaultSchedulerHooks()}
}

func readScheduleConfigFile(path string) (cliFileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cliFileConfig{}, err
	}
	var probe struct {
		Config json.RawMessage `json:"config"`
	}
	// 从默认值起解析，缺省字段保持有效默认而非零值
	if err := json.Unmarshal(data, &probe); err == nil && len(probe.Config) > 0 {
		cfg := defaultCLIFileConfig()
		if err := json.Unmarshal(probe.Config, &cfg); err != nil {
			return cliFileConfig{}, fmt.Errorf("解析调度配置失败 %s: %w", path, err)
		}
		return cfg, nil
	}
	cfg := defaultCLIFileConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cliFileConfig{}, fmt.Errorf("解析调度配置失败 %s: %w", path, err)
	}
	return cfg, nil
}

type scheduleConfigUpdate struct {
	ScheduleEnabled *bool
	CronExpression  *string
	CheckInterval   *int
	R2Enabled       *bool
	R2WorkerURL     *string
	R2WorkerToken   *string
	R2Filename      *string
	R2AliasFormat   *string
}

func saveScheduleConfigFields(path string, update scheduleConfigUpdate) error {
	configFileWriteMutex.Lock()
	defer configFileWriteMutex.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败 %s: %w", path, err)
	}
	if root == nil {
		root = map[string]any{}
	}
	target := root
	if raw, exists := root["config"]; exists && raw == nil {
		inner := map[string]any{}
		root["config"] = inner
		target = inner
	} else if inner, ok := root["config"].(map[string]any); ok {
		target = inner
	}
	set := func(key string, value any) {
		target[key] = value
	}
	if update.ScheduleEnabled != nil {
		set("schedule-enabled", *update.ScheduleEnabled)
	}
	if update.CronExpression != nil {
		set("cron-expression", *update.CronExpression)
	}
	if update.CheckInterval != nil {
		set("check-interval", *update.CheckInterval)
	}
	if update.R2Enabled != nil {
		set("r2enabled", *update.R2Enabled)
	}
	if update.R2WorkerURL != nil {
		set("r2workerurl", *update.R2WorkerURL)
	}
	if update.R2WorkerToken != nil {
		set("r2workertoken", *update.R2WorkerToken)
	}
	if update.R2Filename != nil {
		set("r2filename", *update.R2Filename)
	}
	if update.R2AliasFormat != nil {
		set("r2aliasformat", *update.R2AliasFormat)
	}
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return atomicWriteFile(path, encoded, 0600)
}

func validateScheduleConfigUpdate(update *scheduleConfigUpdate) error {
	if update.CronExpression != nil {
		expr := strings.TrimSpace(*update.CronExpression)
		update.CronExpression = &expr
		if expr != "" {
			if _, err := cron.ParseStandard(expr); err != nil {
				return fmt.Errorf("Cron 表达式无效: %w", err)
			}
		}
	}
	if update.R2WorkerURL != nil {
		rawURL := strings.TrimSpace(*update.R2WorkerURL)
		if rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return fmt.Errorf("R2 worker-url 必须是有效的 http/https 地址")
			}
		}
		update.R2WorkerURL = &rawURL
	}
	if update.R2WorkerToken != nil {
		token := strings.TrimSpace(*update.R2WorkerToken)
		if token == "" {
			// 留空表示保留已保存的 token
			update.R2WorkerToken = nil
		} else {
			update.R2WorkerToken = &token
		}
	}
	if update.R2Filename != nil {
		name := strings.TrimSpace(*update.R2Filename)
		if name == "" {
			name = "cfdata-targets.txt"
		}
		update.R2Filename = &name
	}
	if update.R2AliasFormat != nil {
		alias := strings.TrimSpace(*update.R2AliasFormat)
		if utf8.RuneCountInString(alias) > 200 {
			return fmt.Errorf("R2 别名模板过长（最多 200 字符）")
		}
		if strings.Contains(alias, "#") {
			return fmt.Errorf("R2 别名模板不能包含 # 字符")
		}
		update.R2AliasFormat = &alias
	}
	return nil
}

func scheduleConfigPayload(configPath string) (map[string]interface{}, error) {
	cfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		return nil, err
	}
	plan := normalizeSchedulePlan(cfg)
	payload := map[string]interface{}{
		"configPath":      configPath,
		"scheduleEnabled": cfg.ScheduleEnabled,
		"cronExpression":  cfg.CronExpression,
		"checkInterval":   cfg.CheckInterval,
		"r2Enabled":       cfg.R2Enabled,
		"r2WorkerUrl":     cfg.R2WorkerURL,
		// token 不回传前端，仅提示是否已保存
		"r2WorkerToken":            "",
		"r2WorkerTokenSet":         strings.TrimSpace(cfg.R2WorkerToken) != "",
		"r2Filename":               cfg.R2Filename,
		"r2AliasFormat":            cfg.R2AliasFormat,
		"effectiveMode":            string(plan.Mode),
		"effectiveIntervalMinutes": int(plan.Interval / time.Minute),
		"warning":                  plan.Warning,
	}
	if webScheduler != nil {
		payload["status"] = webScheduler.Status()
	}
	return payload, nil
}

func (s *scanScheduler) loadPlan() (schedulePlan, error) {
	cfg, err := readScheduleConfigFile(s.configPath)
	if err != nil {
		return schedulePlan{}, err
	}
	return normalizeSchedulePlan(cfg), nil
}

// 配置读取失败时保留上一次有效计划；相同错误最多每分钟提示一次
func (s *scanScheduler) logReloadError(err error) {
	if err == nil {
		return
	}
	message := err.Error()
	now := s.hooks.now()
	s.mu.Lock()
	shouldLog := message != s.lastReloadErr || now.Sub(s.lastReloadErrAt) >= time.Minute
	if shouldLog {
		s.lastReloadErr = message
		s.lastReloadErrAt = now
	}
	s.mu.Unlock()
	if shouldLog {
		fmt.Printf("[schedule] 读取调度配置失败，保留上一次有效计划: %v\n", err)
	}
}

func (s *scanScheduler) statMtime() (time.Time, bool) {
	info, err := os.Stat(s.configPath)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

func (s *scanScheduler) Start() {
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	// 先记录 mtime 再读取，避免读取后文件又更新时漏掉变更
	mtime, mtimeOK := s.statMtime()
	plan, loadErr := s.loadPlan()
	if loadErr != nil {
		plan = schedulePlan{Mode: scheduleDisabled}
		plan.Warning = fmt.Sprintf("读取调度配置失败: %v", loadErr)
	}
	s.plan = plan
	s.lastMtime, s.mtimeKnown = mtime, mtimeOK
	s.nextRun = s.nextRunFor(s.plan, s.hooks.now())
	s.stopCh = make(chan struct{})
	s.reloadCh = make(chan struct{}, 1)
	s.loopDone = make(chan struct{})
	stopCh, reloadCh, loopDone := s.stopCh, s.reloadCh, s.loopDone
	s.mu.Unlock()

	if loadErr != nil {
		s.logReloadError(loadErr)
	}
	go s.loop(stopCh, reloadCh, loopDone)
	s.logApplied("调度已启动")
}

func (s *scanScheduler) logApplied(prefix string) {
	s.mu.Lock()
	plan, next := s.plan, s.nextRun
	s.mu.Unlock()
	fmt.Printf("[schedule] %s：%s\n", prefix, describeScheduleApplied(plan, next))
}

func (s *scanScheduler) Stop() {
	s.mu.Lock()
	if !s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.stopCh)
	loopDone := s.loopDone
	s.mu.Unlock()

	<-loopDone
	finished := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
	}
}

func (s *scanScheduler) Reload() {
	s.mu.Lock()
	started := s.started && !s.stopped
	reloadCh := s.reloadCh
	s.mu.Unlock()
	if !started {
		return
	}
	// 同步重载，保证保存配置后立即能读到新的状态与下次运行时间
	if !s.reloadPlan() {
		return
	}
	s.setNextRun(s.nextRunFor(s.currentPlan(), s.hooks.now()))
	select {
	case reloadCh <- struct{}{}:
	default:
	}
	s.logApplied("配置已重载")
}

func (s *scanScheduler) currentPlan() schedulePlan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plan
}

func (s *scanScheduler) intervalFor(plan schedulePlan) time.Duration {
	if s.hooks.intervalOverride > 0 {
		return s.hooks.intervalOverride
	}
	return plan.Interval
}

func (s *scanScheduler) nextRunFor(plan schedulePlan, now time.Time) time.Time {
	if !plan.Enabled {
		return time.Time{}
	}
	if plan.Mode == scheduleCron && plan.CronSchedule != nil {
		return plan.CronSchedule.Next(now)
	}
	return now.Add(s.intervalFor(plan))
}

func (s *scanScheduler) setNextRun(next time.Time) {
	s.mu.Lock()
	s.nextRun = next
	s.mu.Unlock()
}

func (s *scanScheduler) reloadPlan() bool {
	// 先取 mtime 再读文件：文件若在两者之间更新，最多多触发一次重载，不会漏掉变更
	mtime, mtimeOK := s.statMtime()
	plan, err := s.loadPlan()
	if err != nil {
		s.logReloadError(err)
		return false
	}
	s.mu.Lock()
	s.plan = plan
	s.lastMtime, s.mtimeKnown = mtime, mtimeOK
	s.mu.Unlock()
	return true
}

func (s *scanScheduler) reloadIfMtimeChanged() bool {
	mtime, ok := s.statMtime()
	s.mu.Lock()
	changed := !s.mtimeKnown || !ok || !mtime.Equal(s.lastMtime)
	s.mu.Unlock()
	if !changed {
		return false
	}
	return s.reloadPlan()
}

func (s *scanScheduler) loop(stopCh <-chan struct{}, reloadCh <-chan struct{}, loopDone chan struct{}) {
	var timerC <-chan time.Time
	var stopTimer func()
	pollC, stopPoll := s.hooks.newTicker(s.hooks.pollInterval)
	defer func() {
		stopPoll()
		if stopTimer != nil {
			stopTimer()
		}
		close(loopDone)
	}()

	resetTimer := func(immediate bool) {
		if stopTimer != nil {
			stopTimer()
			stopTimer = nil
			timerC = nil
		}
		plan := s.currentPlan()
		if !plan.Enabled {
			s.setNextRun(time.Time{})
			return
		}
		if immediate && plan.Mode == scheduleInterval {
			s.trigger("启动")
		}
		now := s.hooks.now()
		next := s.nextRunFor(plan, now)
		s.setNextRun(next)
		delay := next.Sub(now)
		if delay <= 0 {
			delay = time.Millisecond
		}
		timerC, stopTimer = s.hooks.newTimer(delay)
	}

	resetTimer(true)
	for {
		select {
		case <-stopCh:
			return
		case <-reloadCh:
			resetTimer(false)
		case <-pollC:
			if s.reloadIfMtimeChanged() {
				resetTimer(false)
				s.logApplied("配置文件变更，已重载")
			}
		case <-timerC:
			timerC = nil
			if stopTimer != nil {
				stopTimer()
				stopTimer = nil
			}
			if plan := s.currentPlan(); plan.Enabled {
				s.trigger("定时")
			}
			resetTimer(false)
		}
	}
}

func (s *scanScheduler) trigger(reason string) {
	if !s.running.CompareAndSwap(false, true) {
		s.recordTrigger(reason, "跳过：上一次调度任务仍在运行")
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.running.Store(false)
		defer func() {
			if recovered := recover(); recovered != nil {
				fmt.Printf("[schedule] 调度任务异常: %v\n", recovered)
				s.recordTrigger(reason, fmt.Sprintf("失败：调度任务异常: %v", recovered))
			}
		}()
		if s.hooks.busy != nil && s.hooks.busy() {
			s.recordTrigger(reason, "跳过：已有任务运行")
			return
		}
		if s.hooks.run == nil {
			s.recordTrigger(reason, "跳过：未配置扫描执行器")
			return
		}
		s.mu.Lock()
		s.lastRun = s.hooks.now()
		s.mu.Unlock()
		result := strings.TrimSpace(s.hooks.run(reason))
		if result == "" {
			result = "完成"
		}
		s.recordTrigger(reason, result)
	}()
}

func (s *scanScheduler) recordTrigger(reason, result string) {
	s.mu.Lock()
	s.lastResult = result
	s.mu.Unlock()
}

func (s *scanScheduler) Status() schedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := schedulerStatus{
		Enabled:        s.plan.Enabled,
		Mode:           string(s.plan.Mode),
		CronExpression: s.plan.CronExpression,
		Warning:        s.plan.Warning,
		Running:        s.running.Load(),
		LastResult:     s.lastResult,
	}
	if s.plan.Mode == scheduleInterval {
		status.IntervalMinutes = int(s.plan.Interval / time.Minute)
	}
	if !s.nextRun.IsZero() {
		status.NextRun = s.nextRun.Format(time.RFC3339)
	}
	if !s.lastRun.IsZero() {
		status.LastRun = s.lastRun.Format(time.RFC3339)
	}
	return status
}

var webScheduler *scanScheduler

func loadScheduledCLIConfig(configPath string) (*cliConfig, error) {
	fileCfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		return nil, err
	}
	cfg := &cliConfig{}
	provided := map[string]bool{}
	// 调度产出始终为 Sub-Store 目标列表，忽略配置文件中的 outformat
	fileCfg.Format = "substore"
	applyCLIFileConfigFields(cfg, fileCfg, provided)
	if err := resolveCLIExportFields(cfg, fileCfg, configPath, provided); err != nil {
		return nil, err
	}
	if err := validateCLIOutputConfig(cfg); err != nil {
		return nil, err
	}
	cfg.configResolved = true
	return cfg, nil
}

// 调度运行按配置文件选择测速地址，且不改写进程级 speedTestURL
func scheduledSpeedURL(cfg *cliConfig) string {
	if strings.EqualFold(strings.TrimSpace(cfg.mode), "nsb") {
		return cfg.nsbURL
	}
	return cfg.offURL
}

func runScheduledScan(configPath string) string {
	if anyTaskRunning() {
		fmt.Println("[schedule] 跳过本次触发：已有任务运行中")
		return "跳过：已有任务运行"
	}
	cfg, err := loadScheduledCLIConfig(configPath)
	if err != nil {
		recordProgramDebugError("scheduled_scan_config", err.Error())
		fmt.Printf("[schedule] 调度配置加载失败: %v\n", err)
		return "失败：配置加载失败"
	}
	if cfg.noColor {
		disableANSIColors()
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.mode))
	switch mode {
	case "official":
		err = prepareOfficialCLIConfig(cfg)
	case "nsb":
		err = prepareNSBCLIConfig(cfg)
	default:
		err = fmt.Errorf("不支持的 mode: %s（仅支持 official 或 nsb）", cfg.mode)
	}
	if err != nil {
		recordProgramDebugError("scheduled_scan_config", err.Error())
		fmt.Printf("[schedule] 调度配置无效: %v\n", err)
		return "失败：" + err.Error()
	}

	fmt.Printf("[schedule] 开始调度扫描（模式 %s）\n", mode)
	session := newCLISession(cfg)
	speedURL := scheduledSpeedURL(cfg)
	var pipelineErr error
	if err := session.runTaskSync(func(ctx context.Context, session *appSession) {
		if mode == "official" {
			pipelineErr = runOfficialPipeline(ctx, session, cfg, speedURL)
			return
		}
		pipelineErr = runNSBPipeline(ctx, session, cfg, speedURL)
	}); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("[schedule] 跳过本次触发：已有任务运行中")
			return "跳过：已有任务运行"
		}
		recordProgramDebugError("scheduled_scan", err.Error())
		fmt.Printf("[schedule] 调度扫描启动失败: %v\n", err)
		return "失败：" + err.Error()
	}
	if pipelineErr != nil {
		recordProgramDebugError("scheduled_scan", pipelineErr.Error())
		fmt.Printf("[schedule] 调度扫描失败: %v\n", pipelineErr)
		return "失败：" + pipelineErr.Error()
	}
	fmt.Println("[schedule] 调度扫描完成")
	return "完成"
}

func startWebScheduler(configPath string) *scanScheduler {
	scheduler := newScanScheduler(configPath)
	scheduler.hooks.busy = anyTaskRunning
	scheduler.hooks.run = func(string) string {
		return runScheduledScan(configPath)
	}
	scheduler.Start()
	webScheduler = scheduler

	if !scheduler.Status().Enabled {
		fmt.Println("自动调度: 未启用（可在 Web 设置中开启）")
	}
	return scheduler
}

func reloadWebScheduler() {
	if webScheduler != nil {
		webScheduler.Reload()
	}
}

func describeScheduleApplied(plan schedulePlan, next time.Time) string {
	timezone := scheduleTimezoneLabel()
	if !plan.Enabled {
		return fmt.Sprintf("时区 %s，%s", timezone, describeSchedulePlan(plan))
	}
	if next.IsZero() {
		return fmt.Sprintf("时区 %s，%s，下一次触发未安排", timezone, describeSchedulePlan(plan))
	}
	return fmt.Sprintf("时区 %s，%s，下一次触发 %s", timezone, describeSchedulePlan(plan), next.Format("2006-01-02 15:04:05"))
}

func scheduleTimezoneLabel() string {
	name, offset := time.Now().Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	utc := fmt.Sprintf("UTC%s%02d:%02d", sign, offset/3600, offset%3600/60)
	if name == "" || name == fmt.Sprintf("%s%02d", sign, offset/3600) {
		return utc
	}
	return fmt.Sprintf("%s (%s)", name, utc)
}
