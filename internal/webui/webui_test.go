package webui

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestHandlerServesLiveOpenAPIAndSPA(t *testing.T) {
	calls := 0
	handler, err := NewHandler(func() ([]byte, error) {
		calls++
		return []byte("openapi: 3.0.3\nx-generated: " + strconv.Itoa(calls) + "\n"), nil
	}, "v1.2.3")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	for wantCall := 1; wantCall <= 2; wantCall++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("openapi status: %d", recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), "x-generated: "+strconv.Itoa(wantCall)) {
			t.Fatalf("expected live generation %d, got %q", wantCall, recorder.Body.String())
		}
		if recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("expected no-store cache header")
		}
	}

	versionRecorder := httptest.NewRecorder()
	handler.ServeHTTP(versionRecorder, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if versionRecorder.Code != http.StatusOK || versionRecorder.Body.String() != "{\"version\":\"v1.2.3\"}\n" {
		t.Fatalf("unexpected version response: status %d body %q", versionRecorder.Code, versionRecorder.Body.String())
	}
	if versionRecorder.Header().Get("Content-Type") != "application/json; charset=utf-8" || versionRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected version headers: %#v", versionRecorder.Header())
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/get/api/v1/users", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<div id=\"app\"></div>") {
		t.Fatalf("expected SPA fallback, got status %d body %q", recorder.Code, recorder.Body.String())
	}
}
