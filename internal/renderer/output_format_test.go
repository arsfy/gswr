package renderer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang-openapi/internal/model"
)

func TestWriteJSON(t *testing.T) {
	ir := &model.IR{
		Title:   "T",
		Version: "1.0.0",
		Routes: []model.Route{
			{
				Method:      "GET",
				Path:        "/ping",
				OperationID: "ping",
				Responses: []model.Response{
					{StatusCode: 200, Description: "OK", Schema: &model.Schema{Type: "object"}},
				},
			},
		},
	}

	out := filepath.Join(t.TempDir(), "openapi.json")
	if err := WriteJSON(ir, out); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if decoded["openapi"] != "3.0.3" {
		t.Fatalf("unexpected openapi version: %#v", decoded["openapi"])
	}
}
