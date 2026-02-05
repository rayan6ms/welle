package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"welle/internal/format/astfmt"
)

func TestFormat_GoldenToken(t *testing.T) {
	runGolden(t, filepath.Join("testdata", "token"), func(src string) (string, error) {
		return Format(src, Options{Indent: "  "})
	})
}

func TestFormat_GoldenAST(t *testing.T) {
	runGolden(t, filepath.Join("testdata", "ast"), func(src string) (string, error) {
		out, err := astfmt.FormatASTWithIndent([]byte(src), "  ")
		return string(out), err
	})
}

type formatFunc func(string) (string, error)

func runGolden(t *testing.T, root string, formatFn formatFunc) {
	t.Helper()
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	inDir := filepath.Join(root, "in")
	outDir := filepath.Join(root, "out")

	entries, err := os.ReadDir(inDir)
	if err != nil {
		t.Fatalf("read dir %s: %v", inDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		inPath := filepath.Join(inDir, name)
		outPath := filepath.Join(outDir, name)

		inBytes, err := os.ReadFile(inPath)
		if err != nil {
			t.Fatalf("read %s: %v", inPath, err)
		}

		formatted, err := formatFn(string(inBytes))
		if err != nil {
			t.Fatalf("format %s: %v", name, err)
		}
		if !strings.HasSuffix(formatted, "\n") {
			formatted += "\n"
		}

		if update {
			if err := os.WriteFile(outPath, []byte(formatted), 0o644); err != nil {
				t.Fatalf("update %s: %v", outPath, err)
			}
			continue
		}

		outBytes, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read %s: %v", outPath, err)
		}

		if string(outBytes) != formatted {
			t.Fatalf("golden mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", name, string(outBytes), formatted)
		}
	}
}
