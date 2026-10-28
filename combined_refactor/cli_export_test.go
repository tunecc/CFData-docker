package main

import (
	"strings"
	"testing"
)

func formatSubstoreLines(t *testing.T, rows []cliResultRow) []string {
	t.Helper()
	content, err := formatCLIResults(rows, cliExportConfig{Format: "substore"})
	if err != nil {
		t.Fatalf("formatCLIResults: %v", err)
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		t.Fatalf("substore output must end with newline: %q", content)
	}
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func TestFormatSubstoreResultsIPv4(t *testing.T) {
	rows := []cliResultRow{{
		"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛杉矶",
	}}
	lines := formatSubstoreLines(t, rows)
	if len(lines) != 1 || lines[0] != "1.2.3.4:443#LAX-洛杉矶" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestFormatSubstoreResultsIPv6AlwaysBracketed(t *testing.T) {
	rows := []cliResultRow{{
		"ip": "2606:4700::1", "port": "443", "dc": "HKG", "city": "香港",
	}}
	lines := formatSubstoreLines(t, rows)
	if len(lines) != 1 || lines[0] != "[2606:4700::1]:443#HKG-香港" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestFormatSubstoreResultsAliasFallback(t *testing.T) {
	cases := []struct {
		row  cliResultRow
		want string
	}{
		{cliResultRow{"ip": "1.2.3.4", "port": "443", "dc": "NRT"}, "1.2.3.4:443#NRT"},
		{cliResultRow{"ip": "1.2.3.4", "port": "443", "city": "东京"}, "1.2.3.4:443#东京"},
		{cliResultRow{"ip": "1.2.3.4", "port": "443", "loc": "HKG"}, "1.2.3.4:443#HKG"},
		{cliResultRow{"ip": "1.2.3.4", "port": "443"}, "1.2.3.4:443#1.2.3.4"},
	}
	for _, tc := range cases {
		lines := formatSubstoreLines(t, []cliResultRow{tc.row})
		if len(lines) != 1 || lines[0] != tc.want {
			t.Fatalf("row %#v: got %#v want %q", tc.row, lines, tc.want)
		}
	}
}

func TestFormatSubstoreResultsAliasSanitized(t *testing.T) {
	lines := formatSubstoreLines(t, []cliResultRow{{
		"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛#杉\n矶   ",
	}})
	if len(lines) != 1 || lines[0] != "1.2.3.4:443#LAX-洛杉 矶" {
		t.Fatalf("unexpected lines: %#v", lines)
	}

	lines = formatSubstoreLines(t, []cliResultRow{{
		"ip": "1.2.3.4", "port": "443", "city": "#\n\t ",
	}})
	if len(lines) != 1 || lines[0] != "1.2.3.4:443#1.2.3.4" {
		t.Fatalf("unexpected lines for empty sanitized alias: %#v", lines)
	}
}

func TestFormatSubstoreResultsOmitsEmptyPort(t *testing.T) {
	lines := formatSubstoreLines(t, []cliResultRow{
		{"ip": "1.2.3.4", "port": "0", "dc": "LAX"},
		{"ip": "2606:4700::1", "dc": "HKG"},
	})
	if len(lines) != 2 {
		t.Fatalf("unexpected lines: %#v", lines)
	}
	if lines[0] != "1.2.3.4#LAX" {
		t.Fatalf("unexpected IPv4 no-port line: %q", lines[0])
	}
	if lines[1] != "[2606:4700::1]#HKG" {
		t.Fatalf("unexpected IPv6 no-port line: %q", lines[1])
	}
}

func TestFormatSubstoreResultsSkipsEmptyTarget(t *testing.T) {
	lines := formatSubstoreLines(t, []cliResultRow{
		{"ip": "", "ipport": "", "dc": "LAX"},
		{"ip": "1.2.3.4", "port": "443", "dc": "LAX"},
	})
	if len(lines) != 1 || lines[0] != "1.2.3.4:443#LAX" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestFormatSubstoreResultsParsesIPPortFallback(t *testing.T) {
	cases := []struct {
		row  cliResultRow
		want string
	}{
		{cliResultRow{"ip": "", "ipport": "1.2.3.4:8443", "dc": "LAX"}, "1.2.3.4:8443#LAX"},
		{cliResultRow{"ip": "", "ipport": "[2606:4700::1]:443", "dc": "HKG"}, "[2606:4700::1]:443#HKG"},
		{cliResultRow{"ip": "", "ipport": "2606:4700::1", "dc": "HKG"}, "[2606:4700::1]#HKG"},
		{cliResultRow{"ip": "", "ipport": "1.2.3.4", "dc": "LAX"}, "1.2.3.4#LAX"},
	}
	for _, tc := range cases {
		lines := formatSubstoreLines(t, []cliResultRow{tc.row})
		if len(lines) != 1 || lines[0] != tc.want {
			t.Fatalf("row %#v: got %#v want %q", tc.row, lines, tc.want)
		}
	}

	invalid := []cliResultRow{
		{"ip": "", "ipport": ":", "dc": "LAX"},
		{"ip": "", "ipport": ":443", "dc": "LAX"},
		{"ip": "", "ipport": "1.2.3.4:", "dc": "LAX"},
	}
	if lines := formatSubstoreLines(t, invalid); len(lines) != 0 {
		t.Fatalf("invalid ipport values should be skipped: %#v", lines)
	}
}

func TestFormatSubstoreResultsIPv6AlwaysBracketedWithV6BracketDisabled(t *testing.T) {
	content, err := formatCLIResults([]cliResultRow{
		{"ip": "2606:4700::1", "port": "443", "dc": "HKG"},
	}, cliExportConfig{Format: "substore", V6Bracket: false})
	if err != nil {
		t.Fatalf("formatCLIResults: %v", err)
	}
	if content != "[2606:4700::1]:443#HKG\n" {
		t.Fatalf("substore must ignore outv6bracket=false: %q", content)
	}
}

func TestFormatSubstoreResultsAppliesCustomFields(t *testing.T) {
	rows := []cliResultRow{{
		"ip": "1.2.3.4", "port": "443", "dc": "LAX", "city": "洛杉矶",
	}}
	content, err := formatCLIResults(rows, cliExportConfig{
		Format: "substore",
		Custom: "dc:HKG,city:香港",
	})
	if err != nil {
		t.Fatalf("formatCLIResults: %v", err)
	}
	if content != "1.2.3.4:443#HKG-香港\n" {
		t.Fatalf("custom fields must affect substore alias: %q", content)
	}
}

func TestFormatSubstoreResultsHonorsRowLimit(t *testing.T) {
	rows := []cliResultRow{
		{"ip": "1.1.1.1", "port": "443", "dc": "LAX"},
		{"ip": "2.2.2.2", "port": "443", "dc": "NRT"},
		{"ip": "3.3.3.3", "port": "443", "dc": "HKG"},
	}
	cfg := cliConfig{outIPType: "all", outQualified: "all", startRow: 2, endRow: 3}
	limited := applyCLIExportFilters(&cfg, rows, 0)
	lines := formatSubstoreLines(t, limited)
	if len(lines) != 2 || lines[0] != "2.2.2.2:443#NRT" || lines[1] != "3.3.3.3:443#HKG" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}
