package versioncheck

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCheckLatestRelease(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/repos/arsfy/gswr/releases/latest" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.2.0"}`)),
			Header:     make(http.Header),
		}, nil
	})}
	result, err := (Client{BaseURL: "https://example.test", HTTPClient: client}).Check(context.Background(), "v1.1.0", "arsfy", "gswr")
	if err != nil || result.Latest != "v1.2.0" || !result.UpdateAvailable {
		t.Fatalf("unexpected result %#v, err %v", result, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCompareVersions(t *testing.T) {
	if compareVersions("v1.10.0", "v1.9.9") <= 0 || compareVersions("v1.0.0", "v1.0.0") != 0 {
		t.Fatal("semantic version comparison failed")
	}
}
