package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPassesForACleanTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cmd", "jobs-cli", "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(root, "internal", "a", "a.go"), "package a\n\nfunc A() {}\n")
	if err := check(root); err != nil {
		t.Fatalf("check = %v, want clean trees to pass", err)
	}
}

func TestCheckRejectsPerToolAgentFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cmd", "x", "main.go"), "package main\n")
	for _, name := range forbiddenAgentFiles {
		writeFile(t, filepath.Join(root, name), "tool config\n")
		err := check(root)
		if err == nil || !strings.Contains(err.Error(), "forbidden per-tool agent file: "+name+" (use AGENTS.md)") {
			t.Fatalf("check = %v, want rejection of %s", err, name)
		}
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckCountsMatchThePosixGrepSemantics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cmd", "x", "main.go"),
		"//go:build tools\n\n// real comment\npackage main\n\n// nolint directive lines are code\n//nolint:gocritic\nfunc main() {\n\t// indented comment\n}\n")
	writeFile(t, filepath.Join(root, "internal", "b", "b.go"), "package b\n\nfunc F() {}\n")
	writeFile(t, filepath.Join(root, "internal", "b", "b_test.go"), "// comments in tests are exempt\npackage b\n")
	codeLines, commentLines, err := countSources(root)
	if err != nil {
		t.Fatal(err)
	}
	// main.go: 8 non-blank lines (go:build, 2 comments, package, 2 nolint lines, func, closing)…
	// exact expectation: codeLines = 8+2 = 10, commentLines = 3 (// real comment, the nolint prose line, // indented comment).
	if codeLines != 10 {
		t.Fatalf("codeLines = %d, want 10 (non-blank lines across non-test sources)", codeLines)
	}
	if commentLines != 3 {
		t.Fatalf("commentLines = %d, want 3 (//go: and //nolint prefixes are code, not comments)", commentLines)
	}
}

func TestCheckFailsWhenTheCommentBudgetIsExceeded(t *testing.T) {
	root := t.TempDir()
	var source strings.Builder
	source.WriteString("// one\n// two\n// three\n")
	for i := 0; i < 10; i++ {
		source.WriteString("func F() {}\n")
	}
	writeFile(t, filepath.Join(root, "cmd", "x", "main.go"), source.String())
	writeFile(t, filepath.Join(root, "internal", "b", "b.go"), "package b\n")
	err := check(root)
	if err == nil || !strings.Contains(err.Error(), "comment budget exceeded: 3 comment lines > 5% of 14 code lines") {
		t.Fatalf("check = %v, want the budget rejection with the exact counts", err)
	}
}

func TestCheckFailsClosedWithoutSourceTrees(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cmd", "x", "main.go"), "package main\n")
	if err := check(root); err == nil {
		t.Fatal("check passed without the internal tree, want fail-closed behavior")
	}
}
