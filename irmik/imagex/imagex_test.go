package imagex

import (
	"bytes"
	"image"
	"image/png"
	"testing"
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
