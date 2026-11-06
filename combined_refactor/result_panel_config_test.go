package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResultPanelSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, resultPanelConfigName)
	enabled := true
	want := resultPanelSettings{
		Source:          "scan",
		Format:          "txt",
		FileName:        "cfdata-results",
		StartRow:        "1",
		EndRow:          "20",
		IPType:          "all",
		Qualified:       "qualified",
		Latency:         "200",
		Loss:            "5",
		FieldOrder:      flexibleStringList{"ipport", "dc", "dcCountry"},
		CustomFields:    "备注:测试版",
		SeparatorPreset: "-",
		SeparatorCustom: "-",
		IPv6Brackets:    &enabled,
	}
	if err := saveResultPanelSettings(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := readResultPanelSettings(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil {
		t.Fatal("read returned nil settings")
	}
	if got.Schema != resultPanelConfigSchema {
		t.Fatalf("schema = %q", got.Schema)
	}
	if got.Source != want.Source || got.Format != want.Format || got.FileName != want.FileName {
		t.Fatalf("字段未按预期保存: %+v", got)
	}
	if len(got.FieldOrder) != 3 || got.FieldOrder[0] != "ipport" || got.FieldOrder[2] != "dcCountry" {
		t.Fatalf("fieldOrder = %v", got.FieldOrder)
	}
	if got.IPv6Brackets == nil || !*got.IPv6Brackets {
		t.Fatalf("ipv6Brackets = %v", got.IPv6Brackets)
	}
	if got.UpdatedAt == "" {
		t.Fatal("updatedAt 为空")
	}
}

func TestReadResultPanelSettingsMissingFile(t *testing.T) {
	got, err := readResultPanelSettings(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if got != nil {
		t.Fatalf("missing file should return nil, got %+v", got)
	}
}

func TestReadResultPanelSettingsCorruptFileFallsBackToEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), resultPanelConfigName)
	if err := os.WriteFile(path, []byte("{not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readResultPanelSettings(path)
	if err != nil {
		t.Fatalf("损坏文件应按空配置处理而不是报错: %v", err)
	}
	if got != nil {
		t.Fatalf("损坏文件应返回空配置，got %+v", got)
	}
}

func TestNormalizeResultPanelSettings(t *testing.T) {
	long := strings.Repeat("x", 4096)
	in := resultPanelSettings{
		FieldOrder:      flexibleStringList{"ipport", "  dc  ", "ipport", "", long, "dcCountry"},
		CustomFields:    long,
		SeparatorCustom: long,
	}
	out := normalizeResultPanelSettings(in)
	if len(out.FieldOrder) != 3 {
		t.Fatalf("fieldOrder 应去重去空并过滤超长项: %v", out.FieldOrder)
	}
	if out.FieldOrder[1] != "dc" {
		t.Fatalf("fieldOrder trim 失败: %v", out.FieldOrder)
	}
	if len(out.CustomFields) != resultPanelMaxTextLen {
		t.Fatalf("customFields 未截断: %d", len(out.CustomFields))
	}
	if len(out.SeparatorCustom) != 64 {
		t.Fatalf("separatorCustom 未截断: %d", len(out.SeparatorCustom))
	}
}

func TestFlexibleStringListIgnoresNonStrings(t *testing.T) {
	var settings resultPanelSettings
	payload := `{"fieldOrder":["ipport",1,null,{"k":"v"},"dc"],"format":"csv"}`
	if err := json.Unmarshal([]byte(payload), &settings); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(settings.FieldOrder) != 2 || settings.FieldOrder[0] != "ipport" || settings.FieldOrder[1] != "dc" {
		t.Fatalf("非字符串项应被忽略: %v", settings.FieldOrder)
	}
	if settings.Format != "csv" {
		t.Fatalf("format = %q", settings.Format)
	}
}
