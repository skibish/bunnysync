package statetracker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/skibish/bunnysync/internal/bunnyclient"
	"golang.org/x/sync/errgroup"
)

const connectionsLimit = 25
const numOfWorkers = 10

type StateTracker struct {
	sync.RWMutex
	files map[string]string // key: filename, value: sha256 hash sum of file

	bc     *bunnyclient.BunnyClient
	w      io.Writer
	dryRun bool
}

func New(bc *bunnyclient.BunnyClient, w io.Writer, dryRun bool) *StateTracker {
	return &StateTracker{
		files:  make(map[string]string),
		bc:     bc,
		w:      w,
		dryRun: dryRun,
	}
}

func (s *StateTracker) Initialize(ctx context.Context) error {
	s.Lock()
	defer s.Unlock()

	dirs := []string{"/"}

	for len(dirs) > 0 {
		dir := dirs[0]

		objects, err := s.bc.List(ctx, dir)
		if err != nil {
			return err
		}

		for _, object := range objects {
			if object.IsDirectory {
				dirs = append(dirs, object.CorrectedPath)
			} else {
				s.files[object.CorrectedPath] = object.Checksum
			}
		}

		dirs = dirs[1:]
	}

	return nil
}

func (s *StateTracker) Sync(ctx context.Context, srcDir string) error {
	err := verifyPath(srcDir)
	if err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)
	paths := make(chan string, connectionsLimit)

	g.Go(func() error {
		defer close(paths)
		return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			select {
			case paths <- path:
			case <-gctx.Done():
				return gctx.Err()
			}
			return nil
		})
	})

	for i := 0; i < numOfWorkers; i++ {
		g.Go(func() error {
			for path := range paths {
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				correctedFilePath := strings.TrimPrefix(path, srcDir)[1:]
				hash := getSha256Hash(b)
				remoteHash, ok := s.get(correctedFilePath)
				if !ok || hash != remoteHash {
					fmt.Fprintf(s.w, "+ %s\n", correctedFilePath)
					if !s.dryRun {
						err = s.bc.Upload(ctx, correctedFilePath, b)
						if err != nil {
							return fmt.Errorf("failed to upload %q: %w", correctedFilePath, err)
						}
					}
				}

				s.delete(correctedFilePath)
				if gctx.Err() != nil {
					return gctx.Err()
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}

	if err := s.cleanup(ctx); err != nil {
		return fmt.Errorf("failed to cleanup: %w", err)
	}

	return nil
}

func (s *StateTracker) cleanup(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	semaphore := make(chan struct{}, connectionsLimit)
	for k := range s.files {
		fname := k

		semaphore <- struct{}{}
		g.Go(func() error {
			defer func() {
				<-semaphore
			}()

			fmt.Fprintf(s.w, "- %s\n", fname)
			if !s.dryRun {
				err := s.bc.Delete(ctx, fname)
				if err != nil {
					return fmt.Errorf("failed to delete %q: %w", fname, err)
				}
			}

			if ctx.Err() != nil {
				return ctx.Err()
			}

			return nil
		})
	}

	return g.Wait()
}

func (s *StateTracker) get(path string) (hash string, ok bool) {
	s.RLock()
	defer s.RUnlock()

	hash, ok = s.files[path]
	return
}

func (s *StateTracker) delete(path string) {
	s.Lock()
	defer s.Unlock()

	delete(s.files, path)
}

func verifyPath(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}

	return nil
}

func getSha256Hash(b []byte) string {
	hash := sha256.Sum256(b)

	return strings.ToUpper(hex.EncodeToString(hash[:]))
}
