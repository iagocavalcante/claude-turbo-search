package commands

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"claude-turbo-search/memorydb/internal/sync"
)

// CmdConfig handles `memorydb config {get|set|unset}`.
func (a *App) CmdConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: config {get|set|unset} [--remote URL] [--token TOKEN]")
	}
	switch args[0] {
	case "get", "show":
		return a.cmdConfigGet()
	case "set":
		return a.cmdConfigSet(args[1:])
	case "unset", "clear":
		return a.cmdConfigUnset()
	default:
		return fmt.Errorf("unknown config subcommand: %q", args[0])
	}
}

func (a *App) cmdConfigGet() error {
	path, err := sync.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := sync.Load(path)
	if err != nil {
		return err
	}
	fmt.Printf("config_path: %s\n", path)
	if cfg.Remote == "" && cfg.Token == "" {
		fmt.Println("(no config set — run `memorydb config set --remote URL --token TOKEN`)")
		return nil
	}
	fmt.Printf("remote: %s\n", cfg.Remote)
	fmt.Printf("token:  %s\n", sync.MaskToken(cfg.Token))
	if cfg.Name != "" {
		fmt.Printf("name:   %s (manual override)\n", cfg.Name)
	}
	return nil
}

func (a *App) cmdConfigSet(args []string) error {
	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	remote := fs.String("remote", "", "remote URL (e.g., https://my-app.fly.dev)")
	token := fs.String("token", "", "bearer token")
	name := fs.String("name", "", "manual repo name override (sticks across pushes)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := sync.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := sync.Load(path)
	if err != nil {
		return err
	}
	if r := strings.TrimSpace(*remote); r != "" {
		cfg.Remote = r
	}
	if t := strings.TrimSpace(*token); t != "" {
		cfg.Token = t
	}
	// Allow blanking the name with --name "" by checking flag presence.
	if isFlagSet(fs, "name") {
		cfg.Name = strings.TrimSpace(*name)
	}
	if cfg.Remote == "" || cfg.Token == "" {
		return errors.New("--remote and --token are required (at least once)")
	}
	if err := sync.Save(path, cfg); err != nil {
		return err
	}
	fmt.Printf("Config saved to %s\n", path)
	return nil
}

func isFlagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func (a *App) cmdConfigUnset() error {
	path, err := sync.DefaultPath()
	if err != nil {
		return err
	}
	if err := sync.Clear(path); err != nil {
		return err
	}
	fmt.Println("Config cleared.")
	return nil
}

// CmdPull downloads the remote memory.db into the local repo.
// Refuses to overwrite a non-empty local DB unless force is true.
func (a *App) CmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite an existing local memory.db")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := sync.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := sync.Load(path)
	if err != nil {
		return err
	}
	if cfg.Remote == "" || cfg.Token == "" {
		return errors.New("no sync remote configured. run `memorydb config set --remote URL --token TOKEN`")
	}

	originURL, err := sync.OriginURL(a.RepoRoot)
	if err != nil {
		return fmt.Errorf("could not determine repo origin: %w", err)
	}
	slug := sync.Slug(originURL)

	if a.dbExists() && !*force {
		info, _ := os.Stat(a.DBFile)
		if info != nil && info.Size() > 0 {
			return fmt.Errorf("local memory.db already exists at %s (%d bytes). pass --force to overwrite", a.DBFile, info.Size())
		}
	}

	if err := os.MkdirAll(a.MemoryDir, 0o755); err != nil {
		return err
	}
	if err := sync.Pull(sync.PullOptions{
		Remote: cfg.Remote, Token: cfg.Token, Slug: slug, DBPath: a.DBFile,
	}); err != nil {
		return err
	}
	fmt.Printf("Pulled %s/api/repos/%s/db -> %s\n", strings.TrimRight(cfg.Remote, "/"), slug, a.DBFile)
	return nil
}

// CmdPush uploads the local memory.db to the configured remote.
func (a *App) CmdPush() error {
	path, err := sync.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := sync.Load(path)
	if err != nil {
		return err
	}
	if cfg.Remote == "" || cfg.Token == "" {
		return errors.New("no sync remote configured. run `memorydb config set --remote URL --token TOKEN`")
	}
	if !a.dbExists() {
		return fmt.Errorf("no memory database at %s — nothing to push", a.DBFile)
	}
	originURL, err := sync.OriginURL(a.RepoRoot)
	if err != nil {
		return fmt.Errorf("could not determine repo origin: %w", err)
	}
	slug := sync.Slug(originURL)

	name, source := cfg.Name, "manual"
	if name == "" {
		name = sync.RepoName(originURL)
		source = "auto"
	}

	if err := sync.Push(sync.PushOptions{
		Remote:     cfg.Remote,
		Token:      cfg.Token,
		Slug:       slug,
		DBPath:     a.DBFile,
		Name:       name,
		NameSource: source,
	}); err != nil {
		return err
	}
	fmt.Printf("Pushed %s to %s/api/repos/%s/push\n", a.DBFile, strings.TrimRight(cfg.Remote, "/"), slug)
	return nil
}
