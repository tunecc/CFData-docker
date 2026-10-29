package main

import "testing"

func TestAppVersionLabel(t *testing.T) {
	oldVersion, oldRevision := appVersion, appRevision
	defer func() {
		appVersion, appRevision = oldVersion, oldRevision
	}()

	cases := []struct {
		version  string
		revision string
		want     string
	}{
		{"dev", "unknown", "dev"},
		{"dev", "", "dev"},
		{"dev", "ec745dd1234567890", "sha-ec745dd"},
		{"dev", "sha-ec745dd", "sha-ec745dd"},
		{"edge-ec745dd", "ec745dd1234567890", "sha-ec745dd"},
		{"v1.2.3", "ec745dd1234567890", "v1.2.3 (sha-ec745dd)"},
		{"v1.2.3", "unknown", "v1.2.3"},
	}
	for _, tc := range cases {
		appVersion, appRevision = tc.version, tc.revision
		if got := appVersionLabel(); got != tc.want {
			t.Errorf("appVersionLabel() with version=%q revision=%q = %q, want %q", tc.version, tc.revision, got, tc.want)
		}
	}
}
