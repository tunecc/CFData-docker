package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTaskConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}
	return path
}

func readTaskConfigTarget(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}
	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("解析配置文件失败: %v", err)
	}
	if inner, ok := root["config"].(map[string]any); ok {
		return inner
	}
	return root
}

func TestSaveTaskConfigFieldsRoundtrip(t *testing.T) {
	path := writeTaskConfigFile(t, `{"cli":true,"offthreads":100,"r2enabled":true}`)
	threads := 200
	delay := 300
	speedMin := 0.5
	speedLimit := 10
	autoSpeed := true
	scanMode := "httping"
	mode := "nsb"
	outFile := "result.csv"
	sourceURL := "https://example.com/ips.txt"
	relink := true
	relinkHost := "probe.example.com"
	compact := false
	update := taskConfigUpdate{
		Mode:               &mode,
		OfficialScanMode:   ptrString("tcping"),
		OfficialThreads:    &threads,
		OfficialDelay:      &delay,
		OfficialSpeedMin:   &speedMin,
		OfficialSpeedLimit: &speedLimit,
		OfficialAutoSpeed:  &autoSpeed,
		NSBScanMode:        &scanMode,
		NSBOutFile:         &outFile,
		NSBSourceURL:       &sourceURL,
		NSBRelink:          &relink,
		NSBRelinkHost:      &relinkHost,
		NSBCompact:         &compact,
	}
	if err := saveTaskConfigFields(path, update); err != nil {
		t.Fatalf("保存任务参数失败: %v", err)
	}
	target := readTaskConfigTarget(t, path)
	if got := target["offthreads"].(float64); int(got) != 200 {
		t.Errorf("offthreads = %v, want 200", got)
	}
	if got := target["offautospeed"].(bool); !got {
		t.Errorf("offautospeed = %v, want true", got)
	}
	if got := target["offscanmode"].(string); got != "tcping" {
		t.Errorf("offscanmode = %v, want tcping", got)
	}
	if got := target["nsbscanmode"].(string); got != "httping" {
		t.Errorf("nsbscanmode = %v, want httping", got)
	}
	if got := target["nsbenablespeed"]; got != nil {
		t.Errorf("未传字段不应写入, got %v", got)
	}
	if target["r2enabled"] != true {
		t.Errorf("既有字段 r2enabled 不应被修改")
	}
	if got := target["nsbsourceurl"].(string); got != sourceURL {
		t.Errorf("nsbsourceurl = %v", got)
	}
	if got := target["relinkhost"].(string); got != relinkHost {
		t.Errorf("relinkhost = %v", got)
	}
}

func TestValidateTaskConfigUpdate(t *testing.T) {
	badPort := 70000
	update := taskConfigUpdate{OfficialPort: &badPort}
	if err := validateTaskConfigUpdate(&update); err == nil {
		t.Errorf("端口 70000 应校验失败")
	}

	badMode := "ftp"
	update = taskConfigUpdate{OfficialScanMode: &badMode}
	if err := validateTaskConfigUpdate(&update); err == nil {
		t.Errorf("扫描方式 ftp 应校验失败")
	}

	badURL := "ftp://example.com/ips"
	update = taskConfigUpdate{NSBSourceURL: &badURL}
	if err := validateTaskConfigUpdate(&update); err == nil {
		t.Errorf("非法网络 URL 应校验失败")
	}

	badFile := "../escape.csv"
	update = taskConfigUpdate{NSBOutFile: &badFile}
	if err := validateTaskConfigUpdate(&update); err == nil {
		t.Errorf("带路径分隔符的输出文件名应校验失败")
	}

	goodThreads := 100
	goodMode := "official"
	update = taskConfigUpdate{OfficialThreads: &goodThreads, Mode: &goodMode}
	if err := validateTaskConfigUpdate(&update); err != nil {
		t.Errorf("合法参数不应校验失败: %v", err)
	}
}

func TestValidateTaskConfigUpdateNormalizesTargetDCAndURL(t *testing.T) {
	dc := " hkg "
	url := " https://example.com/ips.txt "
	outFile := " ip.csv "
	update := taskConfigUpdate{OfficialTargetDC: &dc, NSBSourceURL: &url, NSBOutFile: &outFile}
	if err := validateTaskConfigUpdate(&update); err != nil {
		t.Fatalf("合法参数不应校验失败: %v", err)
	}
	if *update.OfficialTargetDC != "HKG" {
		t.Errorf("目标数据中心应去空格并转大写, got %q", *update.OfficialTargetDC)
	}
	if *update.NSBSourceURL != "https://example.com/ips.txt" {
		t.Errorf("网络 URL 应去首尾空格, got %q", *update.NSBSourceURL)
	}
	if *update.NSBOutFile != "ip.csv" {
		t.Errorf("输出文件名应去首尾空格, got %q", *update.NSBOutFile)
	}
}

func TestSaveTaskConfigFieldsPreservesUnsetKeys(t *testing.T) {
	path := writeTaskConfigFile(t, `{"ghtoken":"secret","nsbtls":true,"cron-expression":"0 4 * * *"}`)
	threads := 50
	update := taskConfigUpdate{NSBThreads: &threads}
	if err := saveTaskConfigFields(path, update); err != nil {
		t.Fatalf("保存任务参数失败: %v", err)
	}
	target := readTaskConfigTarget(t, path)
	if target["ghtoken"] != "secret" {
		t.Errorf("ghtoken 不应被修改")
	}
	if target["nsbtls"] != true {
		t.Errorf("nsbtls 不应被修改")
	}
	if target["cron-expression"] != "0 4 * * *" {
		t.Errorf("调度字段不应被修改")
	}
}

func TestSaveTaskConfigFieldsWritesConfigWrapper(t *testing.T) {
	path := writeTaskConfigFile(t, `{"cli":true,"config":{"ghtoken":"secret"}}`)
	threads := 80
	update := taskConfigUpdate{OfficialThreads: &threads}
	if err := saveTaskConfigFields(path, update); err != nil {
		t.Fatalf("保存任务参数失败: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}
	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("解析配置文件失败: %v", err)
	}
	inner, ok := root["config"].(map[string]any)
	if !ok {
		t.Fatalf("config 包裹层应保持为对象: %v", root)
	}
	if inner["offthreads"].(float64) != 80 {
		t.Errorf("offthreads 应写入 config 包裹层, got %v", inner["offthreads"])
	}
	if inner["ghtoken"] != "secret" {
		t.Errorf("config 包裹层既有字段不应被修改")
	}
	if root["offthreads"] != nil {
		t.Errorf("顶层不应出现 offthreads")
	}
}

func ptrString(v string) *string {
	return &v
}
