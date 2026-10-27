package main

import (
	"encoding/json"
	"net"
	"os"
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

func TestGetIPListContentFallsBackOnCorruptCache(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("ips-v4.txt", []byte("not-an-ip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := getIPListContent("ips-v4.txt", "http://127.0.0.1:1/ips-v4")
	if err != nil {
		t.Fatalf("getIPListContent() error = %v, want embedded fallback", err)
	}

	entries, err := parseIPList(content)
	if err != nil {
		t.Fatalf("parse fallback content: %v", err)
	}
	valid := 0
	for _, entry := range entries {
		if _, _, err := net.ParseCIDR(entry); err == nil {
			valid++
		}
	}
	if valid == 0 {
		t.Fatalf("getIPListContent() returned invalid cached content %q", content)
	}
}

func TestEmbeddedASNSnapshot(t *testing.T) {
	reader, err := openEmbeddedASNReader()
	if err != nil {
		t.Fatalf("openEmbeddedASNReader() error = %v", err)
	}
	defer reader.Close()

	if reader.Metadata.DatabaseType != asnDatabaseType {
		t.Fatalf("embedded ASN database type = %q, want %q", reader.Metadata.DatabaseType, asnDatabaseType)
	}
	if reader.Metadata.BuildEpoch == 0 {
		t.Fatal("embedded ASN database has no build epoch")
	}

	var record asnRecord
	if err := reader.Lookup(net.ParseIP("1.1.1.1"), &record); err != nil {
		t.Fatalf("lookup 1.1.1.1: %v", err)
	}
	if record.AutonomousSystemNumber == 0 || record.AutonomousSystemOrganization == "" {
		t.Fatalf("lookup 1.1.1.1 returned empty ASN record: %+v", record)
	}
}

func TestValidateIPListContent(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
	}{
		{name: "valid v4", filename: "ips-v4.txt", content: "1.1.1.0/24\n2.2.2.2\n"},
		{name: "valid v6", filename: "ips-v6.txt", content: "2606:4700::/32\n"},
		{name: "empty", filename: "ips-v4.txt", content: "", wantErr: true},
		{name: "garbage", filename: "ips-v4.txt", content: "not-an-ip\n", wantErr: true},
		{name: "v6 in v4 file", filename: "ips-v4.txt", content: "2606:4700::/32\n", wantErr: true},
		{name: "v4 in v6 file", filename: "ips-v6.txt", content: "1.1.1.0/24\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIPListContent(tc.filename, tc.content)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateIPListContent() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestParseLocationsContent(t *testing.T) {
	locations, err := parseLocationsContent([]byte(`[{"iata":"HKG","city":"Hong Kong"}]`))
	if err != nil {
		t.Fatalf("parse valid locations: %v", err)
	}
	if len(locations) != 1 || locations[0].Iata != "HKG" {
		t.Fatalf("unexpected locations: %+v", locations)
	}

	if _, err := parseLocationsContent([]byte(`[]`)); err == nil {
		t.Fatal("empty locations should be rejected")
	}
	if _, err := parseLocationsContent([]byte(`{`)); err == nil {
		t.Fatal("corrupt locations JSON should be rejected")
	}
}
