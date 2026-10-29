package main

import (
	"bytes"
	"html"
	"strings"
)

const versionLinePlaceholder = `<div class="version-line" id="versionLine">dev</div>`

func renderIndexHTML(page []byte, versionLabel string) []byte {
	replacement := `<div class="version-line" id="versionLine">` + html.EscapeString(versionLabel) + `</div>`
	return bytes.Replace(page, []byte(versionLinePlaceholder), []byte(replacement), 1)
}

const (
	updateRepo       = "tunecc/CFData-docker"
	updateRepoAPI    = "https://api.github.com/repos/" + updateRepo
	releaseLatestURL = "https://github.com/" + updateRepo + "/releases/latest"
)

var (
	appVersion  = "dev"
	appRevision = "unknown"
)

func shortAppRevision() string {
	rev := strings.TrimSpace(appRevision)
	rev = strings.TrimPrefix(rev, "sha-")
	if rev == "" || rev == "unknown" {
		return ""
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	return rev
}

// appVersionLabel 优先展示与镜像 sha-<commit> 标签一致的标识
func appVersionLabel() string {
	rev := shortAppRevision()
	if rev == "" {
		return appVersion
	}
	if isStableVersionTag(appVersion) {
		return appVersion + " (sha-" + rev + ")"
	}
	return "sha-" + rev
}

func isStableVersionTag(tag string) bool {
	if len(tag) < 2 || tag[0] != 'v' {
		return false
	}
	parts := strings.Split(tag[1:], ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func pickLatestVersionTag(tags []string) string {
	best := ""
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if !isStableVersionTag(tag) {
			continue
		}
		if best == "" || versionIsOlder(best, tag) {
			best = tag
		}
	}
	return best
}
