package island

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Chunk is one Vite build.manifest entry.
//
// Expected Vite manifest.json shape (Vite 4 root / Vite 5+.vite/):
//
//	{
//	  "islands/Counter.tsx": {
//	    "file": "assets/Counter-abc123.js",
//	    "src": "islands/Counter.tsx",
//	    "isEntry": true,
//	    "css": ["assets/Counter-def456.css"],
//	    "imports": ["_shared-xyz.js"]
//	  },
//	  "_shared-xyz.js": {
//	    "file": "assets/shared-xyz.js"
//	  }
//	}
//
// Keys are usually source paths (src). Named rollup inputs may also appear as
// "Counter" → same chunk shape. Lookup accepts either form.
type Chunk struct {
	File    string   `json:"file"`
	Src     string   `json:"src,omitempty"`
	Name    string   `json:"name,omitempty"`
	IsEntry bool     `json:"isEntry,omitempty"`
	CSS     []string `json:"css,omitempty"`
	Imports []string `json:"imports,omitempty"` // keys into the same manifest
	Dynamic []string `json:"dynamicImports,omitempty"`
}

// Manifest maps Vite source / chunk keys to build outputs.
type Manifest map[string]Chunk

// LoadManifest reads and parses a Vite manifest.json file.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseManifest(data)
}

// ParseManifest unmarshals Vite manifest JSON bytes.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("island: parse manifest: %w", err)
	}
	if m == nil {
		m = Manifest{}
	}
	return m, nil
}

// LoadManifestFromOutDir tries common Vite manifest locations under outDir:
//   - outDir/manifest.json
//   - outDir/.vite/manifest.json (Vite 5+)
func LoadManifestFromOutDir(outDir string) (Manifest, string, error) {
	candidates := []string{
		filepath.Join(outDir, "manifest.json"),
		filepath.Join(outDir, ".vite", "manifest.json"),
	}
	var errs []string
	for _, p := range candidates {
		m, err := LoadManifest(p)
		if err == nil {
			return m, p, nil
		}
		if !os.IsNotExist(err) {
			return nil, p, err
		}
		errs = append(errs, p)
	}
	return nil, "", fmt.Errorf("island: manifest not found (tried %s)", strings.Join(errs, ", "))
}

// Lookup finds a chunk for an island component name (e.g. "Counter").
// It matches, in order:
//  1. exact manifest key
//  2. islands/<Name>.{tsx,jsx,ts,js}
//  3. <Name>.{tsx,jsx,ts,js}
//  4. any entry whose base name (sans extension) equals Name
func (m Manifest) Lookup(name string) (Chunk, string, bool) {
	if m == nil || name == "" {
		return Chunk{}, "", false
	}
	if c, ok := m[name]; ok {
		return c, name, true
	}
	for _, ext := range []string{".tsx", ".jsx", ".ts", ".js"} {
		for _, key := range []string{
			"islands/" + name + ext,
			name + ext,
		} {
			if c, ok := m[key]; ok {
				return c, key, true
			}
		}
	}
	want := strings.ToLower(name)
	for key, c := range m {
		base := filepath.Base(key)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if strings.EqualFold(base, want) {
			return c, key, true
		}
		if c.Src != "" {
			sbase := filepath.Base(c.Src)
			sbase = strings.TrimSuffix(sbase, filepath.Ext(sbase))
			if strings.EqualFold(sbase, want) {
				return c, key, true
			}
		}
		if c.Name != "" && strings.EqualFold(c.Name, want) {
			return c, key, true
		}
	}
	return Chunk{}, "", false
}

// EntryScripts returns the entry JS file plus CSS files for name, resolving
// transitive imports' CSS. The entry module itself is enough for ES modules
// (shared JS chunks load via import); CSS must be linked explicitly.
func (m Manifest) EntryAssets(name string) (js string, css []string, err error) {
	chunk, key, ok := m.Lookup(name)
	if !ok {
		return "", nil, fmt.Errorf("island: %q not found in manifest", name)
	}
	if chunk.File == "" {
		return "", nil, fmt.Errorf("island: manifest entry %q has empty file", key)
	}
	seenCSS := map[string]struct{}{}
	var collect func(Chunk)
	collect = func(c Chunk) {
		for _, href := range c.CSS {
			if href == "" {
				continue
			}
			if _, ok := seenCSS[href]; ok {
				continue
			}
			seenCSS[href] = struct{}{}
			css = append(css, href)
		}
		for _, imp := range c.Imports {
			if next, ok := m[imp]; ok {
				collect(next)
			}
		}
	}
	collect(chunk)
	return chunk.File, css, nil
}
