package lifecycle

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Signals returns a context that cancels on SIGINT/SIGTERM.
// The returned stop function restores the default signal behavior.
func Signals(parent context.Context) (ctx context.Context, stop context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

// Serve starts srv and shuts it down gracefully when ctx is cancelled.
// shutdownTimeout bounds the Shutdown call; zero uses 10s.
func Serve(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			_ = srv.Close()
			return err
		}
		return <-errCh
	}
}
