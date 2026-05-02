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

func findRepoRoot() (string, error) {
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
			return "", fmt.Errorf("failed to locate git repo root")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
