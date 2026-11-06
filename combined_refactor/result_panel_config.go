package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 结果操作面板的显示设置。原先只保存在浏览器 localStorage，按 origin 隔离，
// 换访问地址/浏览器/设备就会丢失；这里把它落到数据目录，随容器数据保留。
const (
	resultPanelConfigSchema = "cfdata.result-panel.v1"
	resultPanelConfigName   = "result-panel.json"
	resultPanelMaxFieldSeq  = 128
	resultPanelMaxTextLen   = 2048
	resultPanelMaxRowsLen   = 32
)

// flexibleStringList 容忍非字符串元素：只保留字符串项，其余忽略。
type flexibleStringList []string

func (l *flexibleStringList) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		*l = nil
		return nil
	}
	out := make(flexibleStringList, 0, len(raw))
	for _, item := range raw {
		if string(item) == "null" {
			continue // JSON null 会被解码成空字符串，这里按非字符串忽略
		}
		var value string
		if err := json.Unmarshal(item, &value); err == nil {
			out = append(out, value)
		}
	}
	*l = out
	return nil
}

func (l flexibleStringList) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(l))
}

type resultPanelSettings struct {
	Schema          string             `json:"schema,omitempty"`
	Source          string             `json:"source,omitempty"`
	Format          string             `json:"format,omitempty"`
	FileName        string             `json:"fileName,omitempty"`
	StartRow        string             `json:"startRow,omitempty"`
	EndRow          string             `json:"endRow,omitempty"`
	IPType          string             `json:"ipType,omitempty"`
	Qualified       string             `json:"qualified,omitempty"`
	Latency         string             `json:"latency,omitempty"`
	Loss            string             `json:"loss,omitempty"`
	FieldOrder      flexibleStringList `json:"fieldOrder,omitempty"`
	CustomFields    string             `json:"customFields,omitempty"`
	SeparatorPreset string             `json:"separatorPreset,omitempty"`
	SeparatorCustom string             `json:"separatorCustom,omitempty"`
	IPv6Brackets    *bool              `json:"ipv6Brackets,omitempty"`
	UpdatedAt       string             `json:"updatedAt,omitempty"`
}

func defaultResultPanelConfigPath() string {
	return filepath.Join(filepath.Dir(defaultCLIConfigPath()), resultPanelConfigName)
}

// readResultPanelSettings 读取服务器设置。文件不存在时返回 (nil, nil)，
// 调用方据此判断“服务器还没有保存过设置”。
//
// 文件损坏或不可读时同样按空配置处理并返回 (nil, nil)：不阻止启动，前端会
// 用浏览器里现有的设置重新写入，从而自动修复损坏文件。
func readResultPanelSettings(path string) (*resultPanelSettings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		fmt.Printf("[result-panel] 读取设置文件失败，按空配置处理: %v\n", err)
		return nil, nil
	}
	var settings resultPanelSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Printf("[result-panel] 设置文件损坏，按空配置处理: %v\n", err)
		return nil, nil
	}
	normalized := normalizeResultPanelSettings(settings)
	return &normalized, nil
}

func saveResultPanelSettings(path string, settings resultPanelSettings) error {
	normalized := normalizeResultPanelSettings(settings)
	normalized.Schema = resultPanelConfigSchema
	normalized.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(path, data, 0644)
}

func normalizeResultPanelSettings(settings resultPanelSettings) resultPanelSettings {
	trim := func(value string, limit int) string {
		value = strings.TrimSpace(value)
		if len(value) > limit {
			value = value[:limit]
		}
		return value
	}
	settings.Schema = resultPanelConfigSchema
	settings.Source = trim(settings.Source, 32)
	settings.Format = trim(settings.Format, 32)
	settings.FileName = trim(settings.FileName, 256)
	settings.StartRow = trim(settings.StartRow, resultPanelMaxRowsLen)
	settings.EndRow = trim(settings.EndRow, resultPanelMaxRowsLen)
	settings.IPType = trim(settings.IPType, 32)
	settings.Qualified = trim(settings.Qualified, 32)
	settings.Latency = trim(settings.Latency, resultPanelMaxRowsLen)
	settings.Loss = trim(settings.Loss, resultPanelMaxRowsLen)
	settings.CustomFields = trim(settings.CustomFields, resultPanelMaxTextLen)
	settings.SeparatorPreset = trim(settings.SeparatorPreset, 32)
	settings.SeparatorCustom = trim(settings.SeparatorCustom, 64)

	keys := make(flexibleStringList, 0, len(settings.FieldOrder))
	seen := make(map[string]bool, len(settings.FieldOrder))
	for _, key := range settings.FieldOrder {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > 64 || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
		if len(keys) >= resultPanelMaxFieldSeq {
			break
		}
	}
	settings.FieldOrder = keys
	return settings
}
