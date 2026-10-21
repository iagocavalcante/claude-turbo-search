package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"claude-turbo-search/memorydb/internal/commands"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, commands.Usage())
		os.Exit(1)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	scriptDir, err := executableDir()
	if err != nil {
		fatal(err)
	}

	app := commands.New(repoRoot, scriptDir)
	if err := app.Execute(os.Args[1], os.Args[2:]); err != nil {
		if err.Error() == "unknown command" {
			fmt.Fprintln(os.Stderr, commands.Usage())
			os.Exit(1)
		}
		fatal(err)
	}
}

func executableDir() (string, error) {
	if scriptDir := os.Getenv("MEMORY_SCRIPT_DIR"); scriptDir != "" {
		return scriptDir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd), nil
}

// findRepoRoot handles normal checkouts, submodules and linked worktrees.
func findRepoRoot() (string, error) {
	target := os.Getenv("MEMORY_REPO_ROOT")
	if target == "" {
		target = "."
	}
	out, err := exec.Command("git", "-C", target, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %s", target)
	}
	root := strings.TrimSpace(string(out))
	if os.Getenv("MEMORY_REPO_ROOT") != "" {
		absolute, err := filepath.Abs(target)
		if err != nil {
			return "", err
		}
		absolute, err = filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", err
		}
		if absolute != root {
			return "", fmt.Errorf("MEMORY_REPO_ROOT must point to a repository root")
		}
	}
	return root, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
