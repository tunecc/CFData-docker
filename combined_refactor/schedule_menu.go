package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
)

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

func scheduleMenuSummary(configPath string) string {
	cfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		return "未配置"
	}
	plan := normalizeSchedulePlan(cfg)
	switch plan.Mode {
	case scheduleCron:
		return "Cron " + plan.CronExpression
	case scheduleInterval:
		return fmt.Sprintf("每 %d 分钟", int(plan.Interval/time.Minute))
	default:
		return "已关闭"
	}
}

func scheduleNextRunTime(plan schedulePlan, now time.Time) (time.Time, bool) {
	if !plan.Enabled {
		return time.Time{}, false
	}
	if plan.Mode == scheduleCron && plan.CronSchedule != nil {
		return plan.CronSchedule.Next(now), true
	}
	if plan.Mode == scheduleInterval {
		return now.Add(plan.Interval), true
	}
	return time.Time{}, false
}

func printScheduleStatus(configPath string) {
	cfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		fmt.Printf("  读取配置失败: %v\n", err)
		return
	}
	plan := normalizeSchedulePlan(cfg)
	fmt.Printf("  调度状态: %s\n", describeSchedulePlan(plan))
	fmt.Printf("  配置文件: %s\n", configPath)
	if next, ok := scheduleNextRunTime(plan, time.Now()); ok {
		fmt.Printf("  下一次触发: %s\n", next.Format("2006-01-02 15:04:05"))
	}
	if plan.Enabled && plan.Mode == scheduleInterval {
		fmt.Printf("  检测间隔: %d 分钟\n", int(plan.Interval/time.Minute))
	}
}

func runScheduleMenu(configPath string) {
	for {
		cfg, err := readScheduleConfigFile(configPath)
		sectionHeader("自动调度")
		if err != nil {
			fmt.Printf("  读取配置失败: %v\n", err)
		} else {
			fmt.Printf("  当前状态: %s\n", describeSchedulePlan(normalizeSchedulePlan(cfg)))
		}
		fmt.Println("  1. 查看调度状态")
		fmt.Println("  2. 设置 Cron 表达式")
		fmt.Println("  3. 设置检测间隔（分钟）")
		fmt.Println("  4. 启用/停用调度")
		fmt.Println("  5. 启动常驻调度")
		fmt.Println("  0. 返回菜单")
		fmt.Print("请选择> ")
		line, outcome := readMenuLine()
		if outcome != promptOK {
			return
		}
		switch strings.TrimSpace(line) {
		case "1":
			printScheduleStatus(configPath)
		case "2":
			configureScheduleCron(configPath)
		case "3":
			configureScheduleInterval(configPath)
		case "4":
			toggleScheduleEnabled(configPath)
		case "5":
			runScheduleResident(configPath)
		case "0":
			return
		default:
			fmt.Println("请输入 0-5 之间的数字")
		}
	}
}

func configureScheduleCron(configPath string) {
	cfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		fmt.Printf("  读取配置失败: %v\n", err)
		return
	}
	value, outcome := askLine("Cron 表达式", displayOrEmpty(cfg.CronExpression),
		"5 段格式：分 时 日 月 周，例如 0 4 * * * 表示每天 04:00；留空表示改用检测间隔")
	if outcome != promptOK {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		if err := saveScheduleConfigFields(configPath, scheduleConfigUpdate{CronExpression: &value}); err != nil {
			fmt.Printf("  保存失败: %v\n", err)
			return
		}
		fmt.Println("  已清空 Cron 表达式，将按检测间隔调度")
		return
	}
	if _, err := cron.ParseStandard(value); err != nil {
		fmt.Printf("  Cron 表达式无效: %v\n", err)
		return
	}
	if err := saveScheduleConfigFields(configPath, scheduleConfigUpdate{CronExpression: &value}); err != nil {
		fmt.Printf("  保存失败: %v\n", err)
		return
	}
	fmt.Printf("  已保存 Cron 表达式: %s\n", value)
}

func configureScheduleInterval(configPath string) {
	cfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		fmt.Printf("  读取配置失败: %v\n", err)
		return
	}
	current := cfg.CheckInterval
	minutes, outcome := askInt("检测间隔（分钟）", fmt.Sprintf("%d", current),
		"<=0 使用默认 2880 分钟，小于 60 分钟按 60 分钟处理；Cron 表达式为空时生效", current, 0, 0)
	if outcome != promptOK {
		return
	}
	if err := saveScheduleConfigFields(configPath, scheduleConfigUpdate{CheckInterval: &minutes}); err != nil {
		fmt.Printf("  保存失败: %v\n", err)
		return
	}
	effective := normalizeScheduleInterval(minutes)
	fmt.Printf("  已保存检测间隔: %d 分钟（实际按 %d 分钟调度）\n", minutes, int(effective/time.Minute))
}

func toggleScheduleEnabled(configPath string) {
	cfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		fmt.Printf("  读取配置失败: %v\n", err)
		return
	}
	options := []string{"启用", "停用"}
	idx, outcome := askChoice("调度开关", "启用后由本进程内调度器按计划自动扫描", options, boolChoiceIndex(cfg.ScheduleEnabled))
	if outcome != promptOK {
		return
	}
	enabled := idx == 0
	if err := saveScheduleConfigFields(configPath, scheduleConfigUpdate{ScheduleEnabled: &enabled}); err != nil {
		fmt.Printf("  保存失败: %v\n", err)
		return
	}
	state := "已启用"
	if !enabled {
		state = "已停用"
	}
	fmt.Printf("  调度%s\n", state)
}

func runScheduleResident(configPath string) {
	cfg, err := readScheduleConfigFile(configPath)
	if err != nil {
		fmt.Printf("  读取配置失败: %v\n", err)
		return
	}
	plan := normalizeSchedulePlan(cfg)
	if !plan.Enabled {
		fmt.Println("  调度未启用，请先在菜单 4 中启用")
		return
	}
	scheduler := newScanScheduler(configPath)
	scheduler.hooks.busy = anyTaskRunning
	scheduler.hooks.run = func(string) string {
		return runScheduledScan(configPath)
	}
	scheduler.Start()
	defer scheduler.Stop()

	status := scheduler.Status()
	fmt.Printf("  常驻调度已启动: %s\n", describeSchedulePlan(plan))
	if status.NextRun != "" {
		fmt.Printf("  下一次触发: %s\n", status.NextRun)
	}
	fmt.Println("  按 Ctrl+C 退出常驻调度")
	waitForSchedulerInterrupt()
	fmt.Println()
	fmt.Println("  正在停止调度器...")
}

func waitForSchedulerInterrupt() {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupts)
	<-interrupts
}
