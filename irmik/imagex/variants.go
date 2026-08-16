package imagex

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Variant is one encoded size from Variants.
type Variant struct {
	Width       int
	Bytes       []byte
	ContentType string
}

// Variants encodes src once per width (aspect preserved). Use at upload time
// so admin thumbnails exist before the first view. Widths must be > 0.
func Variants(src io.Reader, widths []int, opts Options) ([]Variant, error) {
	if len(widths) == 0 {
		return nil, fmt.Errorf("imagex: widths required")
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	if opts.Format == "" {
		opts.Format = WEBP
	}
	out := make([]Variant, 0, len(widths))
	for _, w := range widths {
		if w <= 0 {
			return nil, fmt.Errorf("imagex: invalid width %d", w)
		}
		o := opts
		o.MaxWidth = w
		body, ct, err := Transform(bytes.NewReader(data), o)
		if err != nil {
			return nil, err
		}
		out = append(out, Variant{Width: w, Bytes: body, ContentType: ct})
	}
	return out, nil
}

// WriteVariants writes name-{width}.ext under destDir (e.g. photo-375.webp).
// name is sanitized to a single path segment; extension follows opts.Format.
func WriteVariants(destDir, name string, src io.Reader, widths []int, opts Options) (map[int]string, error) {
	if destDir == "" {
		return nil, fmt.Errorf("imagex: destDir required")
	}
	name = variantBase(name)
	if opts.Format == "" {
		opts.Format = WEBP
	}
	vs, err := Variants(src, widths, opts)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	ext := variantExt(opts.Format)
	paths := make(map[int]string, len(vs))
	for _, v := range vs {
		path := filepath.Join(destDir, name+"-"+strconv.Itoa(v.Width)+ext)
		if err := os.WriteFile(path, v.Bytes, 0o644); err != nil {
			return nil, err
		}
		paths[v.Width] = path
	}
	return paths, nil
}

func variantBase(name string) string {
	name = filepath.Base(name)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return "image"
	}
	return name
}

func variantExt(f Format) string {
	switch f {
	case PNG:
		return ".png"
	case JPEG:
		return ".jpg"
	case WEBP, "":
		return ".webp"
	default:
		return ".img"
	}
}
