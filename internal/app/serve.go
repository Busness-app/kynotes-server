package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/yoshiofthewire/kynotes-server/internal/blobstore"
	"github.com/yoshiofthewire/kynotes-server/internal/config"
	"github.com/yoshiofthewire/kynotes-server/internal/health"
	"github.com/yoshiofthewire/kynotes-server/internal/httpapi"
	"github.com/yoshiofthewire/kynotes-server/internal/logging"
	"github.com/yoshiofthewire/kynotes-server/internal/storage"
)

func Serve(ctx context.Context, c config.Config, log *logging.Logger) error {
	if err := config.Validate(c); err != nil {
		return err
	}
	lockPath := filepath.Join(c.DataDir, ".kynotes.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("data directory is already in use")
	}
	_, _ = lock.WriteString(strconv.Itoa(os.Getpid()))
	_ = lock.Close()
	defer os.Remove(lockPath)
	store, err := storage.Open(filepath.Join(c.DataDir, "kynotes.sqlite"))
	if err != nil {
		return err
	}
	defer store.Close()
	if err := EnsureBootstrapAdmin(store.DB(), c); err != nil {
		return fmt.Errorf("bootstrap admin failed: %w", err)
	}
	blobs, err := blobstore.New(c.DataDir)
	if err != nil {
		return err
	}
	go runGC(ctx, store.DB(), blobs, c)
	h := &health.Checker{Ready: true}
	srv := &http.Server{Addr: c.Server.Bind, Handler: httpapi.NewRouter(log, c.Server.MaxRequestBytes, h.IsReady, store.DB(), blobs, c), ReadHeaderTimeout: parse(c.Server.ReadHeaderTimeout), ReadTimeout: parse(c.Server.ReadTimeout), WriteTimeout: parse(c.Server.WriteTimeout), IdleTimeout: parse(c.Server.IdleTimeout)}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		sh, cancel := context.WithTimeout(context.Background(), parse(c.Server.ShutdownGrace))
		defer cancel()
		return srv.Shutdown(sh)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func runGC(ctx context.Context, db *sql.DB, blobs *blobstore.Store, c config.Config) {
	ticker := time.NewTicker(parse(c.GC.Interval))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_, _ = storage.RunGC(db, blobs, now, time.Duration(parse(c.GC.Retention)), c.GC.Enabled)
		}
	}
}
func parse(s string) time.Duration { d, _ := time.ParseDuration(s); return d }
