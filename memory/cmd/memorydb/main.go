package main

import (
	"fmt"
	"os"
	"path/filepath"

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

// findRepoRoot resolves the repository whose .claude-memory database this
// invocation should read and write.
//
// MEMORY_REPO_ROOT takes precedence because callers frequently run from a
// working directory that is not the target repo: memory-db.sh has to chdir
// into the plugin's Go module for `go run` to resolve, which would otherwise
// make this walk-up find the plugin's own checkout (writing every project's
// memory into a single shared database) or no repo at all when the plugin is
// installed outside a git repo.
func findRepoRoot() (string, error) {
	if root := os.Getenv("MEMORY_REPO_ROOT"); root != "" {
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			return "", fmt.Errorf("MEMORY_REPO_ROOT=%q is not a git repo root: %w", root, err)
		}
		return root, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("failed to locate git repo root (set MEMORY_REPO_ROOT to select the target repo explicitly)")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
