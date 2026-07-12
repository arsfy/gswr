package upgrade

import (
	"bytes"
	"context"
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/arsfy/gswr/internal/versioncheck"
)

func TestRunInstallsSpecificLatestRelease(t *testing.T) {
	info := &debug.BuildInfo{Path: commandPath, Main: debug.Module{Path: modulePath, Version: "v0.1.0"}}
	var out bytes.Buffer
	var gotName string
	var gotArgs []string
	err := runWithBuildInfo(context.Background(), Options{
		InjectedVersion: "dev",
		Out:             &out,
		CheckLatest: func(context.Context, string, string, string) (versioncheck.Result, error) {
			return versioncheck.Result{Latest: "v0.2.0", UpdateAvailable: true}, nil
		},
		CommandContext: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			gotName, gotArgs = name, append([]string(nil), args...)
			return exec.CommandContext(ctx, "go", "version")
		},
	}, info, true)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if gotName != "go" || len(gotArgs) != 2 || gotArgs[0] != "install" || gotArgs[1] != commandPath+"@v0.2.0" {
		t.Fatalf("unexpected command %q %#v", gotName, gotArgs)
	}
	if strings.Contains(strings.Join(gotArgs, " "), "@latest") {
		t.Fatalf("must install concrete release: %#v", gotArgs)
	}
}

func TestRejectsManualBuild(t *testing.T) {
	info := &debug.BuildInfo{Path: commandPath, Main: debug.Module{Path: modulePath, Version: "v0.1.0"}}
	if isGoInstallBuild(info, "v0.1.0") {
		t.Fatal("ldflags build must not be treated as go install")
	}
}
