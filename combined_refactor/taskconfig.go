package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// taskConfigUpdate 保存官方/非标任务参数到配置文件。
// 指针为 nil 表示不修改对应字段。
type taskConfigUpdate struct {
	Mode               *string
	OfficialScanMode   *string
	OfficialIPType     *int
	OfficialPort       *int
	OfficialThreads    *int
	OfficialDelay      *int
	OfficialSpeedMin   *float64
	OfficialSpeedLimit *int
	OfficialAutoSpeed  *bool
	OfficialTargetDC   *string
	OfficialStopCount  *int
	OfficialSpeedURL   *string
	NSBScanMode        *string
	NSBThreads         *int
	NSBDelay           *int
	NSBResultLimit     *int
	NSBFallbackPort    *int
	NSBSpeedTest       *int
	NSBSpeedMin        *float64
	NSBSpeedLimit      *int
	NSBEnableSpeed     *bool
	NSBTargetDC        *string
	NSBTLS             *bool
	NSBCompact         *bool
	NSBOutFile         *string
	NSBSpeedURL        *string
	NSBSourceURL       *string
	NSBRelink          *bool
	NSBRelinkHost      *string
}

// saveTaskConfigFields 把任务参数写入配置文件（键与 CLI 配置共用同一份文件）。
// 扫描方式拆分为 offscanmode / nsbscanmode，避免官方与非标互相覆盖；
// 旧配置仍使用共享的 scanmode 键，读取端兼容两种写法。
func saveTaskConfigFields(path string, update taskConfigUpdate) error {
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
	if update.Mode != nil {
		set("mode", *update.Mode)
	}
	if update.OfficialScanMode != nil {
		set("offscanmode", *update.OfficialScanMode)
	}
	if update.OfficialIPType != nil {
		set("offiptype", *update.OfficialIPType)
	}
	if update.OfficialPort != nil {
		set("offport", *update.OfficialPort)
	}
	if update.OfficialThreads != nil {
		set("offthreads", *update.OfficialThreads)
	}
	if update.OfficialDelay != nil {
		set("offdelay", *update.OfficialDelay)
	}
	if update.OfficialSpeedMin != nil {
		set("offspeedmin", *update.OfficialSpeedMin)
	}
	if update.OfficialSpeedLimit != nil {
		set("offspeedlimit", *update.OfficialSpeedLimit)
	}
	if update.OfficialAutoSpeed != nil {
		set("offautospeed", *update.OfficialAutoSpeed)
	}
	if update.OfficialTargetDC != nil {
		set("offdc", *update.OfficialTargetDC)
	}
	if update.OfficialStopCount != nil {
		set("offstop", *update.OfficialStopCount)
	}
	if update.OfficialSpeedURL != nil {
		set("offurl", *update.OfficialSpeedURL)
	}
	if update.NSBScanMode != nil {
		set("nsbscanmode", *update.NSBScanMode)
	}
	if update.NSBThreads != nil {
		set("nsbthreads", *update.NSBThreads)
	}
	if update.NSBDelay != nil {
		set("nsbdelay", *update.NSBDelay)
	}
	if update.NSBResultLimit != nil {
		set("nsbresultlimit", *update.NSBResultLimit)
	}
	if update.NSBFallbackPort != nil {
		set("nsbfallbackport", *update.NSBFallbackPort)
	}
	if update.NSBSpeedTest != nil {
		set("nsbspeedtest", *update.NSBSpeedTest)
	}
	if update.NSBSpeedMin != nil {
		set("nsbspeedmin", *update.NSBSpeedMin)
	}
	if update.NSBSpeedLimit != nil {
		set("nsbspeedlimit", *update.NSBSpeedLimit)
	}
	if update.NSBEnableSpeed != nil {
		set("nsbenablespeed", *update.NSBEnableSpeed)
	}
	if update.NSBTargetDC != nil {
		set("nsbdc", *update.NSBTargetDC)
	}
	if update.NSBTLS != nil {
		set("nsbtls", *update.NSBTLS)
	}
	if update.NSBCompact != nil {
		set("nsbcompact", *update.NSBCompact)
	}
	if update.NSBOutFile != nil {
		set("out", *update.NSBOutFile)
	}
	if update.NSBSpeedURL != nil {
		set("nsburl", *update.NSBSpeedURL)
	}
	if update.NSBSourceURL != nil {
		set("nsbsourceurl", *update.NSBSourceURL)
	}
	if update.NSBRelink != nil {
		set("relink", *update.NSBRelink)
	}
	if update.NSBRelinkHost != nil {
		set("relinkhost", *update.NSBRelinkHost)
	}
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return atomicWriteFile(path, encoded, 0600)
}

func validateTaskConfigUpdate(update *taskConfigUpdate) error {
	validateScanMode := func(value **string, name string) error {
		mode := strings.TrimSpace(**value)
		if mode != string(scanModeTCPing) && mode != string(scanModeHTTPing) {
			return fmt.Errorf("%s 只能是 tcping 或 httping", name)
		}
		*value = &mode
		return nil
	}
	if update.Mode != nil {
		mode := strings.TrimSpace(*update.Mode)
		if mode != "official" && mode != "nsb" {
			return fmt.Errorf("mode 只能是 official 或 nsb")
		}
		update.Mode = &mode
	}
	if update.OfficialScanMode != nil {
		if err := validateScanMode(&update.OfficialScanMode, "官方扫描方式"); err != nil {
			return err
		}
	}
	if update.NSBScanMode != nil {
		if err := validateScanMode(&update.NSBScanMode, "非标扫描方式"); err != nil {
			return err
		}
	}
	if update.OfficialIPType != nil && *update.OfficialIPType != 4 && *update.OfficialIPType != 6 {
		return fmt.Errorf("IP 类型只能是 4 或 6")
	}
	validatePort := func(port *int, name string) error {
		if *port < 1 || *port > 65535 {
			return fmt.Errorf("%s 必须在 1-65535 之间", name)
		}
		return nil
	}
	if update.OfficialPort != nil {
		if err := validatePort(update.OfficialPort, "官方测试端口"); err != nil {
			return err
		}
	}
	if update.NSBFallbackPort != nil {
		if err := validatePort(update.NSBFallbackPort, "非标备用端口"); err != nil {
			return err
		}
	}
	validateThreads := func(count *int, name string) error {
		if *count < 1 || *count > 100000 {
			return fmt.Errorf("%s 必须在 1-100000 之间", name)
		}
		return nil
	}
	if update.OfficialThreads != nil {
		if err := validateThreads(update.OfficialThreads, "官方并发数量"); err != nil {
			return err
		}
	}
	if update.NSBThreads != nil {
		if err := validateThreads(update.NSBThreads, "非标并发数量"); err != nil {
			return err
		}
	}
	validateDelay := func(ms *int, name string) error {
		if *ms < 0 || *ms > 600000 {
			return fmt.Errorf("%s 必须在 0-600000 毫秒之间", name)
		}
		return nil
	}
	if update.OfficialDelay != nil {
		if err := validateDelay(update.OfficialDelay, "官方扫描合格延迟"); err != nil {
			return err
		}
	}
	if update.NSBDelay != nil {
		if err := validateDelay(update.NSBDelay, "非标扫描合格延迟"); err != nil {
			return err
		}
	}
	validateSpeedMin := func(speed *float64, name string) error {
		if *speed < 0 || *speed > 10000 {
			return fmt.Errorf("%s 必须在 0-10000 MB/s 之间", name)
		}
		return nil
	}
	if update.OfficialSpeedMin != nil {
		if err := validateSpeedMin(update.OfficialSpeedMin, "官方测试合格速度"); err != nil {
			return err
		}
	}
	if update.NSBSpeedMin != nil {
		if err := validateSpeedMin(update.NSBSpeedMin, "非标测试合格速度"); err != nil {
			return err
		}
	}
	validateSpeedLimit := func(limit *int, name string) error {
		if *limit < 0 || *limit > 100000 {
			return fmt.Errorf("%s 必须在 0-100000 之间（0 表示关闭测速）", name)
		}
		return nil
	}
	if update.OfficialSpeedLimit != nil {
		if err := validateSpeedLimit(update.OfficialSpeedLimit, "官方测速结果数量"); err != nil {
			return err
		}
	}
	if update.NSBSpeedLimit != nil {
		if err := validateSpeedLimit(update.NSBSpeedLimit, "非标测速结果数量"); err != nil {
			return err
		}
	}
	if update.NSBSpeedTest != nil && (*update.NSBSpeedTest < 0 || *update.NSBSpeedTest > 100000) {
		return fmt.Errorf("非标测速并发必须在 0-100000 之间")
	}
	if update.OfficialStopCount != nil && (*update.OfficialStopCount < 0 || *update.OfficialStopCount > 1000000) {
		return fmt.Errorf("官方提前结束数量必须在 0-1000000 之间")
	}
	if update.NSBResultLimit != nil && *update.NSBResultLimit < 0 {
		return fmt.Errorf("非标扫描合格数量必须是非负整数")
	}
	if update.OfficialTargetDC != nil {
		dc := strings.ToUpper(strings.TrimSpace(*update.OfficialTargetDC))
		update.OfficialTargetDC = &dc
	}
	if update.NSBTargetDC != nil {
		dc := strings.ToUpper(strings.TrimSpace(*update.NSBTargetDC))
		update.NSBTargetDC = &dc
	}
	if update.OfficialSpeedURL != nil {
		raw := strings.TrimSpace(*update.OfficialSpeedURL)
		update.OfficialSpeedURL = &raw
	}
	if update.NSBSpeedURL != nil {
		raw := strings.TrimSpace(*update.NSBSpeedURL)
		update.NSBSpeedURL = &raw
	}
	if update.NSBOutFile != nil {
		name := strings.TrimSpace(*update.NSBOutFile)
		if name != "" && (strings.ContainsAny(name, "/\\") || strings.Contains(name, "..")) {
			return fmt.Errorf("非标输出文件名不能包含路径分隔符")
		}
		update.NSBOutFile = &name
	}
	if update.NSBSourceURL != nil {
		raw := strings.TrimSpace(*update.NSBSourceURL)
		if raw != "" {
			parsed, err := url.Parse(raw)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return fmt.Errorf("非标网络 URL 必须是有效的 http/https 地址")
			}
		}
		update.NSBSourceURL = &raw
	}
	if update.NSBRelinkHost != nil {
		host := strings.TrimSpace(*update.NSBRelinkHost)
		update.NSBRelinkHost = &host
	}
	return nil
}
