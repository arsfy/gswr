package parser

import "testing"

func TestInferTagFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{path: "/api/v1/admin/user/list", want: "admin-user"},
		{path: "/api/v1/general/profile", want: "general-profile"},
		{path: "/api/v1/auth/login", want: "auth"},
		{path: "/api/agent/templates", want: "agent-templates"},
		{path: "/v2/user/{id}", want: "user"},
		{path: "/", want: ""},
	}

	for _, tc := range cases {
		got := inferTagFromPath(tc.path)
		if got != tc.want {
			t.Fatalf("inferTagFromPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
