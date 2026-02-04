package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type InstallOptions struct {
	BinDir string
}

func Install(opts InstallOptions) error {
	if opts.BinDir == "" {
		opts.BinDir = "bin"
	}

	if err := os.MkdirAll(opts.BinDir, 0o755); err != nil {
		return err
	}

	if err := goBuild("./cmd/welle", filepath.Join(opts.BinDir, "welle")); err != nil {
		return fmt.Errorf("build welle: %w", err)
	}

	if err := goBuild("./cmd/welle-lsp", filepath.Join(opts.BinDir, "welle-lsp")); err != nil {
		return fmt.Errorf("build welle-lsp: %w", err)
	}

	return nil
}

func goBuild(pkg, out string) error {
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
