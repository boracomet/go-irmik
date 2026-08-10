package imagex

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	xwebp "golang.org/x/image/webp"
)

func TestTransformResize(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	var src bytes.Buffer
	if err := png.Encode(&src, img); err != nil {
		t.Fatal(err)
	}
	out, ct, err := Transform(&src, Options{MaxWidth: 50, Format: PNG})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" {
		t.Fatalf("ct=%s", ct)
	}
	decoded, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 50 || decoded.Bounds().Dy() != 25 {
		t.Fatalf("size=%v", decoded.Bounds())
	}
}

func TestTransformWebPRoundTrip(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, image.Black)
		}
	}
	var src bytes.Buffer
	if err := png.Encode(&src, img); err != nil {
		t.Fatal(err)
	}
	out, ct, err := Transform(&src, Options{Format: WEBP, Quality: 80})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/webp" {
		t.Fatalf("ct=%s", ct)
	}
	if len(out) == 0 {
		t.Fatal("empty webp")
	}
	decoded, err := xwebp.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 32 || decoded.Bounds().Dy() != 16 {
		t.Fatalf("size=%v", decoded.Bounds())
	}
}
