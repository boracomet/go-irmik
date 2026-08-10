// Package imagex provides image decode/resize/encode helpers.
//
// Supports JPEG/PNG encode via the standard library, WebP decode via
// golang.org/x/image/webp, and WebP encode via the pure-Go
// github.com/deepteams/webp library (no CGO).
//
// Opt-in: import only when processing images.
package imagex

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/deepteams/webp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Format is an output encoding.
type Format string

const (
	JPEG Format = "jpeg"
	PNG  Format = "png"
	// WEBP is lossy WebP encode (pure Go; Quality maps to encoder quality).
	WEBP Format = "webp"
)

// Options controls resize/encode.
type Options struct {
	// MaxWidth / MaxHeight constrain dimensions (preserving aspect). 0 = unlimited.
	MaxWidth  int
	MaxHeight int
	Format    Format
	// Quality for JPEG/WebP (1-100, default 85).
	Quality int
}

// Transform decodes src (JPEG/PNG/GIF/WebP), optionally resizes, and encodes.
func Transform(src io.Reader, opts Options) ([]byte, string, error) {
	if opts.Format == "" {
		opts.Format = JPEG
	}
	if opts.Quality <= 0 {
		opts.Quality = 85
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, "", err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("imagex: decode: %w", err)
	}
	img = resize(img, opts.MaxWidth, opts.MaxHeight)
	var buf bytes.Buffer
	var ct string
	switch opts.Format {
	case JPEG:
		ct = "image/jpeg"
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: opts.Quality})
	case PNG:
		ct = "image/png"
		err = png.Encode(&buf, img)
	case WEBP:
		ct = "image/webp"
		err = webp.Encode(&buf, img, &webp.Options{Quality: float32(opts.Quality)})
	default:
		return nil, "", fmt.Errorf("imagex: unknown format %q", opts.Format)
	}
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), ct, nil
}

// Resize returns a resized image without encoding.
func Resize(img image.Image, maxW, maxH int) image.Image {
	return resize(img, maxW, maxH)
}

func resize(img image.Image, maxW, maxH int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxW <= 0 && maxH <= 0 {
		return img
	}
	nw, nh := w, h
	if maxW > 0 && nw > maxW {
		nh = nh * maxW / nw
		nw = maxW
	}
	if maxH > 0 && nh > maxH {
		nw = nw * maxH / nh
		nh = maxH
	}
	if nw == w && nh == h {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}
