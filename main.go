package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/skibish/bunnysync/internal/bunnyclient"
	"github.com/skibish/bunnysync/internal/statetracker"
)

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(w io.Writer, args []string) error {
	var (
		srcPath  string
		endpoint string
		password string
		zoneName string
		dryRun   bool
	)

	fs := flag.NewFlagSet("bunnysync", flag.ExitOnError)
	fs.StringVar(&srcPath, "src", "", "path to the directory to sync")
	fs.StringVar(&endpoint, "endpoint", "https://storage.bunnycdn.com", "storage endpoint")
	fs.StringVar(&password, "password", "", "storage password")
	fs.StringVar(&zoneName, "zone-name", "", "storage zone name")
	fs.BoolVar(&dryRun, "dry-run", false, "dry run (performs no changes to remote)")

	err := fs.Parse(args)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if srcPath == "" {
		return fmt.Errorf("src is required")
	}
	if endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if zoneName == "" {
		return fmt.Errorf("zone-name is required")
	}

	absSrcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("failed to construct absolute path: %w", err)
	}
	if !dirExists(absSrcPath) {
		return fmt.Errorf("%q is not a directory or does not exist", absSrcPath)
	}

	ctx, done := context.WithCancel(context.Background())

	go func() {
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

		select {
		case sig := <-signalChannel:
			fmt.Fprintf(w, "received signal: %s\n", sig)
			done()
		case <-ctx.Done():
			fmt.Fprint(w, "closing signal goroutine\n")
			return
		}
	}()

	stateTracker := statetracker.New(
		bunnyclient.New(endpoint, zoneName, password),
		w,
		dryRun,
	)

	err = stateTracker.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to build remote state: %w", err)
	}

	err = stateTracker.Sync(ctx, absSrcPath)
	if err != nil {
		return fmt.Errorf("failed to sync files: %w", err)
	}

	return nil
}

func dirExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}
