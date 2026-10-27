package main

import (
	"encoding/json"
	"testing"
)

func TestVersionIsOlderSkipsEdgeAndDev(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"edge-c43b159", "v1.8.5", false},
		{"edge", "v1.8.5", false},
		{"dev", "v1.8.5", false},
		{"v1.8.4", "v1.8.5", true},
		{"v1.8.5", "v1.8.5", false},
		{"v1.10.0", "v1.9.9", false},
	}
	for _, tc := range cases {
		if got := versionIsOlder(tc.current, tc.latest); got != tc.want {
			t.Errorf("versionIsOlder(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestPickLatestVersionTag(t *testing.T) {
	tags := []string{"v1.8.4", "v1.10.0", "v1.9.2", "v1.10.0-beta.1", "nightly", "v1.10.1"}
	if got := pickLatestVersionTag(tags); got != "v1.10.1" {
		t.Errorf("pickLatestVersionTag() = %q, want v1.10.1", got)
	}
	if got := pickLatestVersionTag([]string{"v1.10.0-beta.1", "nightly", "edge"}); got != "" {
		t.Errorf("pickLatestVersionTag(prerelease only) = %q, want empty", got)
	}
}

func TestFallbackDataAvailable(t *testing.T) {
	content, ok := fallbackData("locations.json")
	if !ok {
		t.Fatal("fallbackData(locations.json) not available")
	}
	var locations []location
	if err := json.Unmarshal([]byte(content), &locations); err != nil {
		t.Fatalf("parse embedded locations: %v", err)
	}
	if len(locations) == 0 {
		t.Fatal("embedded locations is empty")
	}

	for _, name := range []string{"ips-v4.txt", "ips-v6.txt"} {
		content, ok := fallbackData(name)
		if !ok {
			t.Fatalf("fallbackData(%s) not available", name)
		}
		ips, err := parseIPList(content)
		if err != nil {
			t.Fatalf("parse embedded %s: %v", name, err)
		}
		if len(ips) == 0 {
			t.Fatalf("embedded %s is empty", name)
		}
	}

	if _, ok := fallbackData("unknown.txt"); ok {
		t.Fatal("fallbackData(unknown.txt) should be unavailable")
	}
}
