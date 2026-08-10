package slug

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"  Spaces  ", "spaces"},
		{"Café résumé", "cafe-resume"},
		{"İstanbul ve Çiğ Köfte", "istanbul-ve-cig-kofte"},
		{"şğüöçı", "sguoci"},
		{"a---b", "a-b"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := Slugify(tt.in); got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
