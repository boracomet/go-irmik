// Package hmr watches the filesystem and notifies on template/route changes.
package hmr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Event is a simplified change notification.
type Event struct {
	Path string
	Op   string
}

// Options configures the watcher.
type Options struct {
	Dirs       []string
	Extensions []string      // e.g. .html .yaml .go — empty = all
	Debounce   time.Duration // default 150ms
}

// Watch starts watching dirs until ctx is cancelled.
// onChange is invoked (debounced) with the triggering event.
func Watch(ctx context.Context, opts Options, onChange func(Event)) error {
	if opts.Debounce <= 0 {
		opts.Debounce = 150 * time.Millisecond
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	for _, dir := range opts.Dirs {
		if dir == "" {
			continue
		}
		if err := addRecursive(w, dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("hmr: watch %s: %w", dir, err)
		}
	}

	var (
		timer *time.Timer
		pending Event
	)
	fire := func() {
		if onChange != nil {
			onChange(pending)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return err
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if !matchExt(ev.Name, opts.Extensions) {
				continue
			}
			// Watch new directories.
			if ev.Op&fsnotify.Create != 0 {
				if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
					_ = addRecursive(w, ev.Name)
				}
			}
			pending = Event{Path: ev.Name, Op: ev.Op.String()}
			if timer == nil {
				timer = time.AfterFunc(opts.Debounce, fire)
			} else {
				timer.Reset(opts.Debounce)
			}
		}
	}
}

func matchExt(name string, exts []string) bool {
	if len(exts) == 0 {
		return true
	}
	lower := strings.ToLower(name)
	for _, e := range exts {
		e = strings.ToLower(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	return false
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		base := d.Name()
		if base == "node_modules" || base == ".git" || base == "out" || base == ".irmik" {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}
