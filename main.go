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
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(w io.Writer) error {
	var (
		srcPath         string
		storageEndpoint string
		storageApiKey   string
		storageZoneName string
		dryRun          bool
	)

	currentDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fs := flag.NewFlagSet("bunnysync", flag.ExitOnError)
	fs.StringVar(&srcPath, "src", currentDirectory, "source path")
	fs.StringVar(&storageEndpoint, "storage-endpoint", "storage.bunnycdn.com", "storage endpoint")
	fs.StringVar(&storageApiKey, "storage-api-key", "", "storage api key")
	fs.StringVar(&storageZoneName, "storage-zone", "", "storage zone name")
	fs.BoolVar(&dryRun, "dry-run", false, "dry run (performs no changes to remote)")

	err = fs.Parse(os.Args[1:])
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if storageEndpoint == "" || storageApiKey == "" || storageZoneName == "" {
		return fmt.Errorf("storage-endpoint, storage-api-key, and storage-zone are required")
	}

	absSrcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	bunnyClient := bunnyclient.New(storageEndpoint, storageZoneName, storageApiKey)
	stateTracker := statetracker.New(bunnyClient, w, dryRun)

	ctx, done := context.WithCancel(context.Background())

	go func() {
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

		select {
		case sig := <-signalChannel:
			fmt.Fprintf(w, "Received signal: %s\n", sig)
			done()
		case <-ctx.Done():
			fmt.Fprint(w, "closing signal goroutine\n")
			return
		}
	}()

	err = stateTracker.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize state tracker: %w", err)
	}

	err = stateTracker.Sync(ctx, absSrcPath)
	if err != nil {
		return fmt.Errorf("failed to sync files: %w", err)
	}

	return nil
}
