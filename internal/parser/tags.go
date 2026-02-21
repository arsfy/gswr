package parser

import "strings"

var actionTagSegments = map[string]bool{
	"get": true, "list": true, "all": true, "show": true, "detail": true,
	"add": true, "create": true, "new": true,
	"edit": true, "update": true, "put": true, "patch": true,
	"del": true, "delete": true, "remove": true,
	"login": true, "logout": true, "register": true, "verify": true,
	"send": true, "status": true,
}

func inferTagFromPath(path string) string {
	parts := make([]string, 0, 6)
	for _, p := range strings.Split(path, "/") {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "{") {
			continue
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return ""
	}

	start := 0
	if parts[0] == "api" {
		start = 1
	}
	if start < len(parts) && isVersionSegment(parts[start]) {
		start++
	}
	if start >= len(parts) {
		return ""
	}

	first := parts[start]
	if start+1 >= len(parts) {
		return first
	}
	second := parts[start+1]
	if actionTagSegments[strings.ToLower(second)] {
		return first
	}
	return first + "-" + second
}

func isVersionSegment(s string) bool {
	if len(s) < 2 || (s[0] != 'v' && s[0] != 'V') {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
