package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/dashboard"
	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
	"github.com/bauyrzhanospan/why-isekai/internal/stats"
)

func cmdDashboard(args []string, out io.Writer) error {
	fs := newFlags("dashboard")
	output := fs.String("o", "", "output file (default: .git/isekai/dashboard.html, never committed)")
	look := fs.String("look", "", "isekai, corporate or a skin name (default: [ui] look in config)")
	open := fs.Bool("open", false, "open the page in the default browser")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("dashboard: %v (usage: isekai dashboard [-o FILE] [--look NAME] [--open])", err)
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	s, err := stats.Build(root, cfg, time.Now())
	if err != nil {
		return err
	}
	looks, errs := dashboard.Looks(root)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "isekai: skipped", e)
	}
	def := cfg.UI.Look
	if *look != "" {
		def = *look
	}
	found := false
	for _, l := range looks {
		if l.ID == def {
			found = true
		}
	}
	if !found {
		if *look != "" {
			return fmt.Errorf("dashboard: no look %q (built in: isekai, corporate; skins live in %s)", def, dashboard.SkinsDir)
		}
		def = "isekai"
	}
	path := *output
	if path == "" {
		gd, err := gitx.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return err
		}
		path = filepath.Join(gd, "isekai", "dashboard.html")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := dashboard.Render(f, s, looks, def, *look != ""); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintln(out, path)
	if *open {
		return openBrowser(path)
	}
	return nil
}

func openBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return errors.New("could not open a browser; open the file above yourself")
	}
	return nil
}
