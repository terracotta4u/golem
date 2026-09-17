package release

import (
	"strings"

	"golang.org/x/mod/semver"
)

func Newer(latest, current string) bool {
	l, c := canon(latest), canon(current)
	if l == "" || c == "" {
		return false
	}
	return semver.Compare(l, c) > 0
}

func Display(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "dev"
	}
	return strings.TrimPrefix(v, "v")
}

func canon(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" || v == "dev" || v == "none" {
		return ""
	}
	if !semver.IsValid("v" + v) {
		return ""
	}
	return "v" + v
}
