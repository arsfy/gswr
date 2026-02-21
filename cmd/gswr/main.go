package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"gswr/internal/parser"
	"gswr/internal/renderer"

	"github.com/spf13/cobra"
)

var Version = "dev"

func main() {
	var (
		entry            string
		out              string
		format           string
		showVersionShort bool
	)

	rootCmd := &cobra.Command{
		Use:   "gswr",
		Short: "Generate OpenAPI from Echo routes via semantic analysis",
		Long:  "gswr generates OpenAPI documents by parsing Echo routing and handler semantics.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersionShort {
				fmt.Fprintln(cmd.OutOrStdout(), currentVersion())
				return nil
			}

			ir, err := parser.ParseEchoProject(entry)
			if err != nil {
				return fmt.Errorf("parse failed: %w", err)
			}
			actualFormat := resolveFormat(format, out)
			switch actualFormat {
			case "json":
				if err := renderer.WriteJSON(ir, out); err != nil {
					return fmt.Errorf("render failed: %w", err)
				}
			default:
				if err := renderer.WriteYAML(ir, out); err != nil {
					return fmt.Errorf("render failed: %w", err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "generated %s from %s (%d routes)\n", out, entry, len(ir.Routes))
			return nil
		},
		Version: currentVersion(),
	}

	flags := rootCmd.Flags()
	flags.StringVar(&entry, "entry", "cmd/main.go", "entry go file")
	flags.StringVar(&out, "out", "docs/openapi.yaml", "output openapi file")
	flags.StringVar(&format, "format", "auto", "output format: auto|yaml|json")
	flags.BoolVarP(&showVersionShort, "version-short", "v", false, "print version and exit")
	_ = flags.MarkHidden("version-short")

	rootCmd.InitDefaultHelpCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveFormat(format, out string) string {
	f := strings.ToLower(strings.TrimSpace(format))
	switch f {
	case "yaml", "yml", "json":
		if f == "yml" {
			return "yaml"
		}
		return f
	case "", "auto":
		ext := strings.ToLower(filepath.Ext(out))
		if ext == ".json" {
			return "json"
		}
		return "yaml"
	default:
		ext := strings.ToLower(filepath.Ext(out))
		if ext == ".json" {
			return "json"
		}
		return "yaml"
	}
}

func currentVersion() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
