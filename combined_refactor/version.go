package main

import "strings"

const (
	updateRepo       = "tunecc/CFData-docker"
	updateRepoAPI    = "https://api.github.com/repos/" + updateRepo
	releaseLatestURL = "https://github.com/" + updateRepo + "/releases/latest"
)

var appVersion = "dev"

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
