package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var forbiddenAgentFiles = []string{"CLAUDE.md", ".cursorrules", ".windsurfrules", ".clinerules", "GEMINI.md"}

const commentBudgetPercent = 5

const asciiSpace = " \t\v\f\r"

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if err := check(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(root string) error {
	for _, name := range forbiddenAgentFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return fmt.Errorf("forbidden per-tool agent file: %s (use AGENTS.md)", name)
		}
	}
	codeLines, commentLines, err := countSources(root)
	if err != nil {
		return err
	}
	if commentLines > codeLines*commentBudgetPercent/100 {
		return fmt.Errorf("comment budget exceeded: %d comment lines > %d%% of %d code lines", commentLines, commentBudgetPercent, codeLines)
	}
	return nil
}

func countSources(root string) (codeLines, commentLines int, err error) {
	for _, dir := range []string{"cmd", "internal"} {
		walkErr := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, visitErr error) error {
			if visitErr != nil {
				return visitErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSuffix(line, "\r")
				if strings.Trim(line, asciiSpace) == "" {
					continue
				}
				codeLines++
				trimmed := strings.TrimLeft(line, asciiSpace)
				if strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "//go:") && !strings.HasPrefix(trimmed, "//nolint") {
					commentLines++
				}
			}
			return nil
		})
		if walkErr != nil {
			return 0, 0, walkErr
		}
	}
	return codeLines, commentLines, nil
}
