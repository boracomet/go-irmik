// Package storage defines a small object-storage interface with a local filesystem
// implementation. For S3-compatible backends, import irmik/storage/s3x separately
// so the AWS SDK is not linked into default binaries.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound indicates the object does not exist.
var ErrNotFound = errors.New("storage: not found")

// Object holds metadata returned by Stat / Put.
type Object struct {
	Key         string
	Size        int64
	ContentType string
}

// Store is a minimal blob store.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader, contentType string) (Object, error)
	Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
	Delete(ctx context.Context, key string) error
	Stat(ctx context.Context, key string) (Object, error)
}

// Local is a filesystem-backed Store under Root.
type Local struct {
	Root string
}

// OpenLocal creates a Local store, ensuring Root exists.
func OpenLocal(root string) (*Local, error) {
	if root == "" {
		return nil, fmt.Errorf("storage: root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Local{Root: root}, nil
}

func (l *Local) resolve(key string) (string, error) {
	key = strings.TrimPrefix(filepath.ToSlash(key), "/")
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("storage: invalid key %q", key)
	}
	full := filepath.Join(l.Root, filepath.FromSlash(key))
	rel, err := filepath.Rel(l.Root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("storage: invalid key %q", key)
	}
	return full, nil
}

func (l *Local) Put(ctx context.Context, key string, r io.Reader, contentType string) (Object, error) {
	_ = ctx
	path, err := l.resolve(key)
	if err != nil {
		return Object{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Object{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Object{}, err
	}
	n, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return Object{}, copyErr
	}
	if closeErr != nil {
		return Object{}, closeErr
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// optional sidecar for content-type
	_ = os.WriteFile(path+".ctype", []byte(contentType), 0o644)
	return Object{Key: key, Size: n, ContentType: contentType}, nil
}

func (l *Local) Get(ctx context.Context, key string) (io.ReadCloser, Object, error) {
	obj, err := l.Stat(ctx, key)
	if err != nil {
		return nil, Object{}, err
	}
	path, err := l.resolve(key)
	if err != nil {
		return nil, Object{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, Object{}, err
	}
	return f, obj, nil
}

func (l *Local) Delete(ctx context.Context, key string) error {
	_ = ctx
	path, err := l.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	_ = os.Remove(path + ".ctype")
	return err
}

func (l *Local) Stat(ctx context.Context, key string) (Object, error) {
	_ = ctx
	path, err := l.resolve(key)
	if err != nil {
		return Object{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Object{}, ErrNotFound
		}
		return Object{}, err
	}
	ct := "application/octet-stream"
	if b, err := os.ReadFile(path + ".ctype"); err == nil {
		ct = string(b)
	}
	return Object{Key: key, Size: info.Size(), ContentType: ct}, nil
}
