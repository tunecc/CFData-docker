package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func writeScheduleConfigFile(t *testing.T, path string, cfg cliFileConfig) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"config": cfg})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func waitUntil(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

func newTestScheduler(t *testing.T, cfg cliFileConfig) (*scanScheduler, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	writeScheduleConfigFile(t, path, cfg)
	scheduler := newScanScheduler(path)
	scheduler.hooks.pollInterval = 10 * time.Millisecond
	return scheduler, path
}

func TestSchedulerDisabledDoesNotTrigger(t *testing.T) {
	scheduler, _ := newTestScheduler(t, cliFileConfig{CheckInterval: 60})
	var runs atomic.Int32
	scheduler.hooks.intervalOverride = 10 * time.Millisecond
	scheduler.hooks.run = func(string) string {
		runs.Add(1)
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()

	time.Sleep(80 * time.Millisecond)
	status := scheduler.Status()
	if runs.Load() != 0 {
		t.Fatalf("runs = %d, want 0 while disabled", runs.Load())
	}
	if status.Mode != string(scheduleDisabled) {
		t.Fatalf("mode = %q, want %q", status.Mode, scheduleDisabled)
	}
	if status.NextRun != "" {
		t.Fatalf("nextRun = %q, want empty", status.NextRun)
	}
}

func TestSchedulerIntervalTriggersImmediately(t *testing.T) {
	scheduler, _ := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CheckInterval: 60})
	var runs atomic.Int32
	// 间隔远大于断言窗口，确保触发来自启动立即执行而不是 ticker
	scheduler.hooks.intervalOverride = 2 * time.Second
	scheduler.hooks.run = func(string) string {
		runs.Add(1)
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()

	if !waitUntil(300*time.Millisecond, func() bool { return runs.Load() >= 1 }) {
		t.Fatalf("expected immediate startup trigger")
	}
}

func TestSchedulerIntervalRepeats(t *testing.T) {
	scheduler, _ := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CheckInterval: 60})
	var runs atomic.Int32
	scheduler.hooks.intervalOverride = 25 * time.Millisecond
	scheduler.hooks.run = func(string) string {
		runs.Add(1)
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()

	if !waitUntil(time.Second, func() bool { return runs.Load() >= 2 }) {
		t.Fatalf("expected repeating interval trigger, runs = %d", runs.Load())
	}
}

func TestSchedulerCronDoesNotTriggerImmediately(t *testing.T) {
	// 远期表达式，避免测试跨越分钟边界时误触发
	scheduler, _ := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 4 1 1 *"})
	var runs atomic.Int32
	scheduler.hooks.run = func(string) string {
		runs.Add(1)
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()

	time.Sleep(120 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatalf("runs = %d, want 0 before first cron time", runs.Load())
	}
	status := scheduler.Status()
	if status.Mode != string(scheduleCron) {
		t.Fatalf("mode = %q, want %q", status.Mode, scheduleCron)
	}
	if status.NextRun == "" {
		t.Fatalf("cron status should expose next run time")
	}
}

func TestSchedulerBusySkipsTrigger(t *testing.T) {
	scheduler, _ := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CheckInterval: 60})
	var runs atomic.Int32
	scheduler.hooks.intervalOverride = 10 * time.Millisecond
	scheduler.hooks.busy = func() bool { return true }
	scheduler.hooks.run = func(string) string {
		runs.Add(1)
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()

	time.Sleep(100 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatalf("runs = %d, want 0 while busy", runs.Load())
	}
	if !waitUntil(200*time.Millisecond, func() bool {
		return scheduler.Status().LastResult != ""
	}) {
		t.Fatalf("expected busy skip to be recorded in status")
	}
}

func TestSchedulerReloadAppliesNewPlanWithoutTriggering(t *testing.T) {
	scheduler, path := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CheckInterval: 60})
	var runs atomic.Int32
	scheduler.hooks.run = func(string) string {
		runs.Add(1)
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()
	if !waitUntil(time.Second, func() bool { return runs.Load() == 1 }) {
		t.Fatalf("expected one startup trigger, runs = %d", runs.Load())
	}

	writeScheduleConfigFile(t, path, cliFileConfig{ScheduleEnabled: false})
	scheduler.Reload()
	if !waitUntil(time.Second, func() bool {
		return scheduler.Status().Mode == string(scheduleDisabled)
	}) {
		t.Fatalf("reload did not apply disabled plan, mode = %q", scheduler.Status().Mode)
	}
	time.Sleep(100 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("reload must not trigger a scan, runs = %d", runs.Load())
	}
}

func TestSchedulerMtimeChangeReloadsPlan(t *testing.T) {
	scheduler, path := newTestScheduler(t, cliFileConfig{})
	var runs atomic.Int32
	scheduler.hooks.run = func(string) string {
		runs.Add(1)
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()
	if got := scheduler.Status().Mode; got != string(scheduleDisabled) {
		t.Fatalf("initial mode = %q, want %q", got, scheduleDisabled)
	}

	writeScheduleConfigFile(t, path, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 4 * * *"})
	bumped := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, bumped, bumped); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if !waitUntil(time.Second, func() bool {
		return scheduler.Status().Mode == string(scheduleCron)
	}) {
		t.Fatalf("mtime change did not reload plan, mode = %q", scheduler.Status().Mode)
	}
	if runs.Load() != 0 {
		t.Fatalf("mtime reload must not trigger a scan, runs = %d", runs.Load())
	}
}

func TestSchedulerStopEndsLoop(t *testing.T) {
	scheduler, _ := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CheckInterval: 60})
	scheduler.hooks.intervalOverride = 10 * time.Millisecond
	scheduler.hooks.run = func(string) string { return "ok" }
	scheduler.Start()
	done := make(chan struct{})
	go func() {
		scheduler.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Stop did not return in time")
	}
}

func TestNormalizeSchedulePlanDisabled(t *testing.T) {
	plan := normalizeSchedulePlan(cliFileConfig{
		CronExpression: "0 4 * * *",
		CheckInterval:  120,
	})
	if plan.Mode != scheduleDisabled {
		t.Fatalf("mode = %q, want %q", plan.Mode, scheduleDisabled)
	}
	if plan.CronSchedule != nil {
		t.Fatalf("disabled plan should not have a cron schedule")
	}
}

func TestNormalizeSchedulePlanCronPrecedence(t *testing.T) {
	plan := normalizeSchedulePlan(cliFileConfig{
		ScheduleEnabled: true,
		CronExpression:  "0 4 * * *",
		CheckInterval:   120,
	})
	if plan.Mode != scheduleCron {
		t.Fatalf("mode = %q, want %q", plan.Mode, scheduleCron)
	}
	if plan.CronSchedule == nil {
		t.Fatalf("cron plan should have a parsed schedule")
	}
	now := time.Date(2026, 10, 28, 3, 0, 0, 0, time.Local)
	next := plan.CronSchedule.Next(now)
	want := time.Date(2026, 10, 28, 4, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("next = %v, want %v", next, want)
	}
}

func TestNormalizeSchedulePlanInvalidCronFallsBackToInterval(t *testing.T) {
	plan := normalizeSchedulePlan(cliFileConfig{
		ScheduleEnabled: true,
		CronExpression:  "not a cron expression",
		CheckInterval:   120,
	})
	if plan.Mode != scheduleInterval {
		t.Fatalf("mode = %q, want %q", plan.Mode, scheduleInterval)
	}
	if plan.Warning == "" {
		t.Fatalf("expected a warning for invalid cron expression")
	}
	if plan.Interval != 120*time.Minute {
		t.Fatalf("interval = %v, want %v", plan.Interval, 120*time.Minute)
	}
}

func TestNormalizeSchedulePlanIntervalClamp(t *testing.T) {
	cases := []struct {
		name     string
		minutes  int
		expected time.Duration
	}{
		{name: "zero", minutes: 0, expected: 2880 * time.Minute},
		{name: "negative", minutes: -10, expected: 2880 * time.Minute},
		{name: "below minimum", minutes: 30, expected: 60 * time.Minute},
		{name: "exact minimum", minutes: 60, expected: 60 * time.Minute},
		{name: "above minimum", minutes: 120, expected: 120 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := normalizeSchedulePlan(cliFileConfig{
				ScheduleEnabled: true,
				CheckInterval:   tc.minutes,
			})
			if plan.Mode != scheduleInterval {
				t.Fatalf("mode = %q, want %q", plan.Mode, scheduleInterval)
			}
			if plan.Interval != tc.expected {
				t.Fatalf("interval = %v, want %v", plan.Interval, tc.expected)
			}
		})
	}
}

func TestDefaultFileConfigScheduleDefaults(t *testing.T) {
	cfg := defaultCLIFileConfig()
	if cfg.ScheduleEnabled {
		t.Fatalf("schedule should be disabled by default")
	}
	if cfg.CheckInterval != 0 {
		t.Fatalf("check interval default = %d, want 0", cfg.CheckInterval)
	}
	if cfg.R2Enabled {
		t.Fatalf("R2 upload should be disabled by default")
	}
	if cfg.R2Filename != "cfdata-targets.txt" {
		t.Fatalf("R2 filename default = %q, want %q", cfg.R2Filename, "cfdata-targets.txt")
	}
}

func TestRunScheduledScanSkipsWhenTaskRunning(t *testing.T) {
	atomic.StoreInt32(&globalRunningTasks, 1)
	defer atomic.StoreInt32(&globalRunningTasks, 0)

	result := runScheduledScan(filepath.Join(t.TempDir(), "missing.json"))
	if !strings.HasPrefix(result, "跳过") {
		t.Fatalf("result = %q, want skip when a task is running", result)
	}
}

func TestRunScheduledScanReportsConfigError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	result := runScheduledScan(path)
	if !strings.HasPrefix(result, "失败") {
		t.Fatalf("result = %q, want failure for invalid config", result)
	}
}

func TestReadScheduleConfigFileAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	if err := os.WriteFile(path, []byte(`{"config":{"schedule-enabled":true}}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := readScheduleConfigFile(path)
	if err != nil {
		t.Fatalf("readScheduleConfigFile: %v", err)
	}
	if !cfg.ScheduleEnabled {
		t.Fatalf("explicit value lost")
	}
	if cfg.OutStartRow != 1 || cfg.OutEndRow != 20 || cfg.Out != "cfdata-results" {
		t.Fatalf("defaults not applied: start=%d end=%d out=%q", cfg.OutStartRow, cfg.OutEndRow, cfg.Out)
	}
}

func TestSchedulerInvalidConfigKeepsPreviousPlan(t *testing.T) {
	scheduler, path := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 4 * * *"})
	scheduler.hooks.run = func(string) string { return "ok" }
	scheduler.Start()
	defer scheduler.Stop()
	if got := scheduler.Status().CronExpression; got != "0 4 * * *" {
		t.Fatalf("initial cron = %q", got)
	}

	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	scheduler.Reload()
	time.Sleep(50 * time.Millisecond)
	status := scheduler.Status()
	if status.Mode != string(scheduleCron) || status.CronExpression != "0 4 * * *" {
		t.Fatalf("invalid config should keep previous plan: mode=%q cron=%q", status.Mode, status.CronExpression)
	}

	writeScheduleConfigFile(t, path, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 5 * * *"})
	scheduler.Reload()
	if !waitUntil(time.Second, func() bool {
		return scheduler.Status().CronExpression == "0 5 * * *"
	}) {
		t.Fatalf("valid config not reloaded, cron = %q", scheduler.Status().CronExpression)
	}
}

func TestLoadScheduledCLIConfigDoesNotMutateGlobals(t *testing.T) {
	oldSkip, oldDNS, oldForced := skipGeoCheck, customDNSServer, customDNSForced
	oldSpeed, oldWorkers := speedTestURL, speedTestWorkers
	t.Cleanup(func() {
		skipGeoCheck, customDNSServer, customDNSForced = oldSkip, oldDNS, oldForced
		speedTestURL, speedTestWorkers = oldSpeed, oldWorkers
	})
	skipGeoCheck = false
	customDNSServer = "9.9.9.9:53"
	customDNSForced = false
	speedTestURL = "https://sentinel.example/speed"
	speedTestWorkers = 7

	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	root := map[string]any{"config": map[string]any{
		"mode":             "official",
		"skipgeo":          true,
		"dns":              "1.1.1.1:53",
		"offurl":           "https://file.example/speed",
		"debug":            "all",
		"nsbspeedtest":     9,
		"schedule-enabled": true,
		"check-interval":   120,
	}}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := loadScheduledCLIConfig(path)
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	if skipGeoCheck {
		t.Fatal("scheduled config must not overwrite skipGeoCheck")
	}
	if customDNSServer != "9.9.9.9:53" || customDNSForced {
		t.Fatalf("scheduled config must not overwrite DNS globals: dns=%q forced=%v", customDNSServer, customDNSForced)
	}
	if speedTestURL != "https://sentinel.example/speed" {
		t.Fatalf("scheduled config must not overwrite speedTestURL: %q", speedTestURL)
	}
	if speedTestWorkers != 7 {
		t.Fatalf("scheduled config must not overwrite speedTestWorkers: %d", speedTestWorkers)
	}
	if cfg.offURL != "https://file.example/speed" || cfg.speedTest != 9 {
		t.Fatalf("scheduled config not applied to cliConfig: url=%q speedtest=%d", cfg.offURL, cfg.speedTest)
	}
	if got := scheduledSpeedURL(cfg); got != "https://file.example/speed" {
		t.Fatalf("scheduledSpeedURL = %q", got)
	}
}

func TestSchedulerReloadUpdatesStatusSynchronously(t *testing.T) {
	scheduler, path := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 4 * * *"})
	scheduler.hooks.run = func(string) string { return "ok" }
	scheduler.Start()
	defer scheduler.Stop()

	writeScheduleConfigFile(t, path, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 5 * * *"})
	scheduler.Reload()
	status := scheduler.Status()
	if status.CronExpression != "0 5 * * *" {
		t.Fatalf("reload not visible immediately: cron = %q", status.CronExpression)
	}
	if status.NextRun == "" {
		t.Fatalf("reload should refresh next run time")
	}
}

func TestSchedulerRecoversAfterInvalidConfigViaPolling(t *testing.T) {
	scheduler, path := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 4 * * *"})
	scheduler.hooks.run = func(string) string { return "ok" }
	scheduler.Start()
	defer scheduler.Stop()

	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	bumped := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, bumped, bumped); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := scheduler.Status().CronExpression; got != "0 4 * * *" {
		t.Fatalf("invalid config should keep previous plan, cron = %q", got)
	}

	writeScheduleConfigFile(t, path, cliFileConfig{ScheduleEnabled: true, CronExpression: "0 5 * * *"})
	later := time.Now().Add(4 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if !waitUntil(time.Second, func() bool {
		return scheduler.Status().CronExpression == "0 5 * * *"
	}) {
		t.Fatalf("polling did not recover after invalid config, cron = %q", scheduler.Status().CronExpression)
	}
}

func TestSaveScheduleConfigFieldsHandlesNullRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	if err := os.WriteFile(path, []byte("null"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	enabled := true
	update := scheduleConfigUpdate{R2Enabled: &enabled}
	if err := saveScheduleConfigFields(path, update); err != nil {
		t.Fatalf("saveScheduleConfigFields: %v", err)
	}
	cfg, err := readScheduleConfigFile(path)
	if err != nil {
		t.Fatalf("readScheduleConfigFile: %v", err)
	}
	if !cfg.R2Enabled {
		t.Fatalf("saved R2 flag lost: %#v", cfg)
	}
}

func TestSchedulerStartStopImmediately(t *testing.T) {
	scheduler, _ := newTestScheduler(t, cliFileConfig{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		scheduler.Start()
		scheduler.Stop()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Start followed by immediate Stop did not return")
	}
}

func TestNormalizeSchedulePlanTrimsCronExpression(t *testing.T) {
	plan := normalizeSchedulePlan(cliFileConfig{
		ScheduleEnabled: true,
		CronExpression:  "  0 4 * * *  ",
		CheckInterval:   120,
	})
	if plan.Mode != scheduleCron {
		t.Fatalf("mode = %q, want %q (trailing whitespace must not break cron)", plan.Mode, scheduleCron)
	}
	if plan.CronExpression != "0 4 * * *" {
		t.Fatalf("cron expression = %q, want trimmed value", plan.CronExpression)
	}
}

func TestDescribeScheduleAppliedIncludesTimezoneAndNextRun(t *testing.T) {
	plan := normalizeSchedulePlan(cliFileConfig{
		ScheduleEnabled: true,
		CronExpression:  "0 4 * * *",
	})
	next := time.Date(2026, 10, 28, 4, 0, 0, 0, time.Local)
	text := describeScheduleApplied(plan, next)
	if !strings.Contains(text, "UTC+") && !strings.Contains(text, "UTC-") {
		t.Fatalf("description must include a UTC offset: %q", text)
	}
	if !strings.Contains(text, "2026-10-28 04:00:00") {
		t.Fatalf("description must include next trigger: %q", text)
	}

	disabled := describeScheduleApplied(schedulePlan{Mode: scheduleDisabled}, time.Time{})
	if (!strings.Contains(disabled, "UTC+") && !strings.Contains(disabled, "UTC-")) || !strings.Contains(disabled, "已关闭") {
		t.Fatalf("disabled description = %q, want timezone and disabled state", disabled)
	}
}

func TestConfigWritersPreserveConcurrentUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	padding := strings.Repeat("x", 128*1024)
	initial := fmt.Sprintf(`{"config":{"proxy":"http://127.0.0.1:7890","check-interval":59,"unused":"%s"}}`, padding)
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var readErr atomic.Value
	stopReader := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stopReader:
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil {
				readErr.Store(err.Error())
				return
			}
			var root map[string]interface{}
			if err := json.Unmarshal(data, &root); err != nil {
				readErr.Store(err.Error())
				return
			}
		}
	}()

	var writeMu sync.Mutex
	var writeErrs []error
	recordErr := func(err error) {
		if err == nil {
			return
		}
		writeMu.Lock()
		writeErrs = append(writeErrs, err)
		writeMu.Unlock()
	}

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			recordErr(updateProxyInConfigFile(path, fmt.Sprintf(`{"server":"127.0.0.1","port":%d}`, 7000+n)))
		}(i)
		go func(n int) {
			defer wg.Done()
			interval := 60 + n
			recordErr(saveScheduleConfigFields(path, scheduleConfigUpdate{CheckInterval: &interval}))
		}(i)
	}
	wg.Wait()
	close(stopReader)
	<-readerDone

	if err, ok := readErr.Load().(string); ok {
		t.Fatalf("reader observed a partially written config: %s", err)
	}
	if len(writeErrs) > 0 {
		t.Fatalf("concurrent writers returned errors: %v", writeErrs)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var root map[string]map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("config is not valid JSON after concurrent saves: %v", err)
	}
	inner := root["config"]
	interval, ok := inner["check-interval"].(float64)
	if !ok || int(interval) < 60 || int(interval) > 99 {
		t.Fatalf("check-interval = %v, want the value of a completed writer (60..99)", inner["check-interval"])
	}
	proxyRaw, ok := inner["proxy"].(string)
	if !ok {
		t.Fatalf("proxy field lost after concurrent saves: %v", inner["proxy"])
	}
	var proxy map[string]interface{}
	if err := json.Unmarshal([]byte(proxyRaw), &proxy); err != nil {
		t.Fatalf("proxy value is not the JSON written by a writer: %q", proxyRaw)
	}
	port, ok := proxy["port"].(float64)
	if !ok || int(port) < 7000 || int(port) > 7039 {
		t.Fatalf("proxy.port = %v, want the value of a completed writer (7000..7039)", proxy["port"])
	}
	if inner["unused"] != padding {
		t.Fatalf("sentinel field damaged by concurrent saves")
	}
}

func TestSchedulerRecoversFromPanic(t *testing.T) {
	scheduler, _ := newTestScheduler(t, cliFileConfig{ScheduleEnabled: true, CheckInterval: 60})
	var runs atomic.Int32
	scheduler.hooks.intervalOverride = 20 * time.Millisecond
	scheduler.hooks.run = func(string) string {
		if runs.Add(1) == 1 {
			panic("boom")
		}
		return "ok"
	}
	scheduler.Start()
	defer scheduler.Stop()

	if !waitUntil(time.Second, func() bool {
		return strings.Contains(scheduler.Status().LastResult, "异常")
	}) {
		t.Fatalf("panic not recorded, result = %q", scheduler.Status().LastResult)
	}
	if !waitUntil(time.Second, func() bool { return runs.Load() >= 2 }) {
		t.Fatalf("scheduler did not run again after panic, runs = %d", runs.Load())
	}
}
