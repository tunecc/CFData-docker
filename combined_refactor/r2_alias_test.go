package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR2AliasFormatConfigParses(t *testing.T) {
	var withKey cliFileConfig
	if err := json.Unmarshal([]byte(`{"r2enabled":true,"r2filename":"targets.txt","r2aliasformat":"{region}-{dc}"}`), &withKey); err != nil {
		t.Fatalf("unmarshal with r2aliasformat: %v", err)
	}
	if withKey.R2AliasFormat != "{region}-{dc}" {
		t.Errorf("R2AliasFormat = %q", withKey.R2AliasFormat)
	}

	var withoutKey cliFileConfig
	if err := json.Unmarshal([]byte(`{"r2enabled":true,"r2filename":"targets.txt"}`), &withoutKey); err != nil {
		t.Fatalf("unmarshal legacy config without r2aliasformat: %v", err)
	}
	if withoutKey.R2AliasFormat != "" {
		t.Errorf("legacy config R2AliasFormat = %q, want empty", withoutKey.R2AliasFormat)
	}

	if got := defaultCLIFileConfig().R2AliasFormat; got != "" {
		t.Errorf("default R2AliasFormat = %q, want empty", got)
	}
}

func TestApplyR2AliasTemplate(t *testing.T) {
	row := cliResultRow{
		"ip":        "1.2.3.4",
		"port":      "443",
		"ipport":    "1.2.3.4:443",
		"dc":        "LAX",
		"dcCountry": "US",
		"region":    "NA",
		"city":      "洛杉矶",
	}
	cases := []struct {
		name     string
		template string
		want     string
	}{
		{"region-dc", "{region}-{dc}", "NA-LAX"},
		{"dc-city default shape", "{dc}-{city}", "LAX-洛杉矶"},
		{"ipport", "{ipport}", "1.2.3.4:443"},
		{"ip and port", "{ip}:{port}", "1.2.3.4:443"},
		{"dcCountry", "{dcCountry}-{dc}", "US-LAX"},
		{"unknown placeholder literal", "{foo}-{dc}", "{foo}-LAX"},
		{"missing field empty", "{city}-{dc}", "-LAX"},
		{"empty template", "", ""},
		{"whitespace collapsed", "{dc}", "LAX"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testRow := row
			if tc.name == "missing field empty" {
				testRow = cliResultRow{"ip": "1.2.3.4", "port": "443", "ipport": "1.2.3.4:443", "dc": "LAX"}
			}
			if got := applyR2AliasTemplate(tc.template, testRow); got != tc.want {
				t.Errorf("applyR2AliasTemplate(%q) = %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}

func TestFormatSubstoreResultsWithAlias(t *testing.T) {
	rows := []cliResultRow{
		{"ip": "1.2.3.4", "port": "443", "ipport": "1.2.3.4:443", "dc": "LAX", "city": "洛杉矶", "region": "NA"},
	}
	if got, want := formatSubstoreResultsWithAlias(rows, "{region}-{dc}"), "1.2.3.4:443#NA-LAX\n"; got != want {
		t.Errorf("template alias = %q, want %q", got, want)
	}
	if got := formatSubstoreResultsWithAlias(rows, ""); got != formatSubstoreResults(rows) {
		t.Errorf("empty template must match default output, got %q want %q", got, formatSubstoreResults(rows))
	}
	fallbackRows := []cliResultRow{{"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛杉矶"}}
	if got, want := formatSubstoreResultsWithAlias(fallbackRows, "{region}"), "1.2.3.4:443#LAX-洛杉矶\n"; got != want {
		t.Errorf("empty alias result must fall back to default, got %q want %q", got, want)
	}
}

func TestScheduledExportUsesAliasTemplate(t *testing.T) {
	var stored map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		stored = nil
		if err := json.Unmarshal(body, &stored); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "cfdata-config.json")
	cfgData, err := json.Marshal(map[string]any{"config": map[string]any{
		"mode": "official", "outformat": "csv", "outstartrow": 1, "outendrow": 20,
		"r2enabled": true, "r2workerurl": server.URL, "r2workertoken": "secret",
		"r2filename": "targets.txt", "r2aliasformat": "{region}-{dc}",
	}})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, cfgData, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := loadScheduledCLIConfig(path)
	if err != nil {
		t.Fatalf("loadScheduledCLIConfig: %v", err)
	}
	rows := []cliResultRow{
		{"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛杉矶", "region": "NA"},
		{"ip": "2606:4700::1", "port": "443", "dc": "HKG", "city": "香港", "region": "AS"},
	}
	if err := writeCLIExportAndMaybeUpload(cfg, rows, rows, "schedule"); err != nil {
		t.Fatalf("writeCLIExportAndMaybeUpload: %v", err)
	}
	want := "1.2.3.4:443#NA-LAX\n[2606:4700::1]:443#AS-HKG\n"
	if stored["value"] != want {
		t.Fatalf("uploaded payload = %#v, want %q", stored, want)
	}
}

func TestTriggerManualR2UploadUsesAliasTemplate(t *testing.T) {
	var payload map[string]string
	done := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload = nil
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		done <- struct{}{}
	}))
	defer server.Close()

	withR2TestHooks(t, server.Client())
	writeManualR2Config(t, map[string]any{
		"r2enabled":     true,
		"r2workerurl":   server.URL,
		"r2workertoken": "secret",
		"r2filename":    "targets.txt",
		"r2aliasformat": "{dc}-{region}",
	})
	session := &appSession{r2AutoUpload: true}
	rows := []cliResultRow{{"ip": "1.2.3.4", "port": "443", "dc": "LAX", "region": "NA", "city": "洛杉矶"}}

	triggerManualR2Upload(session, rows)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("手动任务未触发 R2 上传")
	}
	if payload["value"] != "1.2.3.4:443#LAX-NA\n" {
		t.Fatalf("上传内容 = %#v", payload)
	}
}

func TestSaveScheduleConfigAliasFormatRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfdata-config.json")
	root := map[string]any{"config": map[string]any{"r2enabled": false, "custom-unknown": "keep"}}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	alias := " {region}-{dc} "
	update := scheduleConfigUpdate{R2AliasFormat: &alias}
	if err := validateScheduleConfigUpdate(&update); err != nil {
		t.Fatalf("validateScheduleConfigUpdate: %v", err)
	}
	if err := saveScheduleConfigFields(path, update); err != nil {
		t.Fatalf("saveScheduleConfigFields: %v", err)
	}
	payload, err := scheduleConfigPayload(path)
	if err != nil {
		t.Fatalf("scheduleConfigPayload: %v", err)
	}
	if got := payload["r2AliasFormat"]; got != "{region}-{dc}" {
		t.Errorf("payload r2AliasFormat = %#v, want {region}-{dc}", got)
	}
	fileCfg, err := readScheduleConfigFile(path)
	if err != nil {
		t.Fatalf("readScheduleConfigFile: %v", err)
	}
	if fileCfg.R2AliasFormat != "{region}-{dc}" {
		t.Errorf("file r2aliasformat = %q", fileCfg.R2AliasFormat)
	}
}

func TestValidateScheduleConfigUpdateAliasFormat(t *testing.T) {
	long := strings.Repeat("{dc}", 51) // 204 字符
	invalid := []string{long, "{dc}#{city}"}
	longChinese := strings.Repeat("数", 201) // 201 个汉字（603 字节）
	invalid = append(invalid, longChinese)
	for _, value := range invalid {
		update := scheduleConfigUpdate{R2AliasFormat: &value}
		if err := validateScheduleConfigUpdate(&update); err == nil {
			t.Errorf("alias %q should be rejected", value)
		}
	}
	exactly200Chinese := strings.Repeat("数", 200)
	validCases := []string{" {dc}-{city} ", exactly200Chinese}
	for _, value := range validCases {
		update := scheduleConfigUpdate{R2AliasFormat: &value}
		if err := validateScheduleConfigUpdate(&update); err != nil {
			t.Errorf("alias of %d runes rejected: %v", len([]rune(value)), err)
		}
	}
	trimmed := " {dc}-{city} "
	update := scheduleConfigUpdate{R2AliasFormat: &trimmed}
	if err := validateScheduleConfigUpdate(&update); err != nil {
		t.Errorf("valid alias rejected: %v", err)
	}
	if *update.R2AliasFormat != "{dc}-{city}" {
		t.Errorf("alias not trimmed: %q", *update.R2AliasFormat)
	}
}
