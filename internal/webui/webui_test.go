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
	})
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

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/get/api/v1/users", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<div id=\"app\"></div>") {
		t.Fatalf("expected SPA fallback, got status %d body %q", recorder.Code, recorder.Body.String())
	}
}
