package parser

import "strings"

func buildServers(basePath string, schemes []string, host string) []string {
	bp := strings.TrimSpace(basePath)
	h := strings.TrimSpace(host)
	hasBasePath := bp != ""
	hasHost := h != ""
	hasSchemes := len(schemes) > 0

	// No related annotations provided: do not emit servers.
	if !hasBasePath && !hasHost && !hasSchemes {
		return nil
	}

	if hasBasePath && !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}
	if bp == "" {
		bp = "/"
	}

	// If host or schemes are missing, keep only relative base path.
	if !hasHost || !hasSchemes {
		if hasBasePath {
			return []string{bp}
		}
		return nil
	}

	out := make([]string, 0, len(schemes))
	seen := map[string]bool{}
	for _, s := range schemes {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		url := s + "://" + h + bp
		if !seen[url] {
			seen[url] = true
			out = append(out, url)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
