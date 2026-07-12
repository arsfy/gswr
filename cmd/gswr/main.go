package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/arsfy/gswr/internal/entrypoint"
	"github.com/arsfy/gswr/internal/parser"
	"github.com/arsfy/gswr/internal/renderer"
	"github.com/arsfy/gswr/internal/upgrade"

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
		Use:          "gswr",
		Short:        "Generate OpenAPI from Echo routes via semantic analysis",
		Long:         "gswr generates OpenAPI documents by parsing Echo routing and handler semantics.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersionShort {
				fmt.Fprintln(cmd.OutOrStdout(), currentVersion())
				return nil
			}

			return generate(cmd, ".", entry, out, format)
		},
		Version: currentVersion(),
	}

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&entry, "entry", "", "entry go file (auto-discovered when omitted)")
	flags.StringVar(&out, "out", "docs/openapi.yaml", "output openapi file")
	flags.StringVar(&format, "format", "auto", "output format: auto|yaml|json")
	flags.BoolVarP(&showVersionShort, "version-short", "v", false, "print version and exit")
	_ = flags.MarkHidden("version-short")

	generateCmd := &cobra.Command{
		Use:     "generate [project-dir]",
		Aliases: []string{"g", "gen"},
		Short:   "Generate OpenAPI and auto-discover func main",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			return generate(cmd, root, entry, out, format)
		},
	}
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade gswr when installed with go install",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return upgrade.Run(cmd.Context(), upgrade.Options{
				InjectedVersion: Version,
				Out:             cmd.OutOrStdout(),
				Err:             cmd.ErrOrStderr(),
			})
		},
	})

	rootCmd.InitDefaultHelpCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(cmd *cobra.Command, root, entry, out, format string) error {
	selected := entry
	if selected == "" {
		var err error
		selected, err = entrypoint.Discover(root)
		if err != nil {
			return fmt.Errorf("entry discovery failed: %w", err)
		}
	}
	ir, err := parser.ParseEchoProject(selected)
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
	fmt.Fprintf(cmd.OutOrStdout(), "generated %s from %s (%d routes)\n", out, selected, len(ir.Routes))
	return nil
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
