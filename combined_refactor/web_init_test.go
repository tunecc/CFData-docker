package main

import (
	"strings"
	"testing"
	"time"
)

func TestRenderIndexHTMLInjectsVersion(t *testing.T) {
	page := []byte(`<html><body><div class="version-line" id="versionLine">dev</div><p>dev</p></body></html>`)
	out := string(renderIndexHTML(page, `v1.0 <build>`))
	if !strings.Contains(out, `<div class="version-line" id="versionLine">v1.0 &lt;build&gt;</div>`) {
		t.Fatalf("version placeholder not injected or not escaped: %s", out)
	}
	if strings.Contains(out, `>dev</div>`) {
		t.Fatalf("placeholder should be replaced: %s", out)
	}
	if !strings.Contains(out, `<p>dev</p>`) {
		t.Fatalf("unrelated dev text must stay unchanged: %s", out)
	}

	unchanged := []byte(`<html><body>no version line</body></html>`)
	if got := string(renderIndexHTML(unchanged, "v1.0")); got != string(unchanged) {
		t.Fatalf("page without placeholder should be unchanged, got %s", got)
	}
}

func TestBuildInitConfigPayloadUsesStartupCache(t *testing.T) {
	oldSpeedURL, oldWorkers, oldDebug, oldSkip := speedTestURL, speedTestWorkers, debugMode, skipGeoCheck
	oldDefault := startupSpeedDefault()
	defer func() {
		speedTestURL, speedTestWorkers, debugMode, skipGeoCheck = oldSpeedURL, oldWorkers, oldDebug, oldSkip
		setStartupSpeedDefault(oldDefault)
	}()

	speedTestURL = autoSpeedURLValue
	speedTestWorkers = 9
	debugMode = true
	skipGeoCheck = false
	setStartupSpeedDefault(cloudflareSpeedURL)

	start := time.Now()
	payload := buildInitConfigPayload()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("building init_config took %s, expected no blocking network work", elapsed)
	}
	if payload["speedTestDefault"] != cloudflareSpeedURL {
		t.Fatalf("speedTestDefault = %v, want startup cache %q", payload["speedTestDefault"], cloudflareSpeedURL)
	}
	if payload["speedTestURL"] != autoSpeedURLValue {
		t.Fatalf("speedTestURL = %v, want %q", payload["speedTestURL"], autoSpeedURLValue)
	}
	if payload["version"] != appVersionLabel() {
		t.Fatalf("version = %v, want %q", payload["version"], appVersionLabel())
	}
	if payload["speedTestWorkers"] != 9 || payload["debug"] != true {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	libraries, ok := payload["dcLibraries"].(map[string]dcLibraryStatus)
	if !ok {
		t.Fatalf("dcLibraries has unexpected type: %T", payload["dcLibraries"])
	}
	if _, ok := libraries["4"]; !ok {
		t.Fatalf("dcLibraries missing IPv4 entry: %+v", libraries)
	}
	if _, ok := libraries["6"]; !ok {
		t.Fatalf("dcLibraries missing IPv6 entry: %+v", libraries)
	}
}
