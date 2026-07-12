package versioncheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const githubAPI = "https://api.github.com"

type Result struct {
	Current         string
	Latest          string
	UpdateAvailable bool
}

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
}

func Check(ctx context.Context, current, owner, repo string) (Result, error) {
	return Client{}.Check(ctx, current, owner, repo)
}

func (c Client) Check(ctx context.Context, current, owner, repo string) (Result, error) {
	latest, err := c.LatestRelease(ctx, owner, repo)
	if err != nil {
		return Result{Current: current}, err
	}
	return Result{Current: current, Latest: latest, UpdateAvailable: compareVersions(latest, current) > 0}, nil
}

func (c Client) LatestRelease(ctx context.Context, owner, repo string) (string, error) {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = githubAPI
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gswr")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("github latest release returned %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.TagName), nil
}

func compareVersions(a, b string) int {
	av, bv := parseVersion(a), parseVersion(b)
	if len(av) == 0 || len(bv) == 0 {
		return 0
	}
	for i := 0; i < len(av) || i < len(bv); i++ {
		var ai, bi int
		if i < len(av) {
			ai = av[i]
		}
		if i < len(bv) {
			bi = bv[i]
		}
		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v), "refs/tags/"), "v")
	if v == "" || v == "dev" || v == "(devel)" {
		return nil
	}
	parts := strings.Split(strings.SplitN(v, "-", 2)[0], ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			out = append(out, 0)
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}
