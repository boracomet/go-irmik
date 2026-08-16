package imagex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// DefaultWidths is the allowlist used when Pipeline.Widths is empty.
var DefaultWidths = []int{375, 768, 1440}

const (
	defaultImgPath  = "/_irmik/img"
	defaultMaxSrc   = 8 << 20
	memCacheLimit   = 64
	cacheControlVal = "public, max-age=86400"
)

// Pipeline is an opt-in responsive image optimizer.
//
// Mount it on the Gin engine and merge FuncMap into MountPages so templates
// can emit srcset. Source files must live under Root; remote URLs are rejected.
type Pipeline struct {
	// Root is the filesystem root for source images (e.g. "./public").
	Root string
	// Path is the handler route (default /_irmik/img).
	Path string
	// Widths is the allowlist of srcset widths (default 375, 768, 1440).
	Widths []int
	// Format is the encoded variant format (default WEBP).
	Format Format
	// Quality for JPEG/WebP (default 80).
	Quality int
	// CacheDir, when set, stores encoded variants on disk.
	CacheDir string
	// MaxSourceBytes caps a source file (default 8 MiB).
	MaxSourceBytes int64

	once   sync.Once
	cache  *variantCache
	allow  map[int]struct{}
	widths []int
	path   string
}

func (p *Pipeline) ready() {
	p.once.Do(func() {
		if p.Path == "" {
			p.Path = defaultImgPath
		}
		p.path = p.Path
		for _, w := range p.Widths {
			if w > 0 {
				p.widths = append(p.widths, w)
			}
		}
		if len(p.widths) == 0 {
			p.widths = append([]int(nil), DefaultWidths...)
		}
		if p.Format == "" {
			p.Format = WEBP
		}
		if p.Quality <= 0 {
			p.Quality = 80
		}
		if p.MaxSourceBytes <= 0 {
			p.MaxSourceBytes = defaultMaxSrc
		}
		p.allow = make(map[int]struct{}, len(p.widths))
		for _, w := range p.widths {
			if w > 0 {
				p.allow[w] = struct{}{}
			}
		}
		p.cache = newVariantCache(p.CacheDir)
	})
}

// Mount registers GET Path on r. Root must be set.
func (p *Pipeline) Mount(r gin.IRoutes) error {
	h, err := p.Handler()
	if err != nil {
		return err
	}
	p.ready()
	r.GET(p.path, h)
	return nil
}

// Handler returns the on-demand optimizer. Query: src=/file.jpg&w=375
func (p *Pipeline) Handler() (gin.HandlerFunc, error) {
	if strings.TrimSpace(p.Root) == "" {
		return nil, fmt.Errorf("imagex: Pipeline.Root is required")
	}
	p.ready()
	return p.serve, nil
}

func (p *Pipeline) serve(c *gin.Context) {
	src := c.Query("src")
	w, err := strconv.Atoi(c.Query("w"))
	if err != nil || w <= 0 {
		c.String(http.StatusBadRequest, "imagex: invalid w")
		return
	}
	if _, ok := p.allow[w]; !ok {
		c.String(http.StatusBadRequest, "imagex: width not allowed")
		return
	}
	path, err := p.resolve(src)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.Status(http.StatusNotFound)
			return
		}
		c.String(http.StatusInternalServerError, "imagex: stat")
		return
	}
	if st.Size() > p.MaxSourceBytes {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}

	key := cacheKey(src, w, p.Format, p.Quality)
	if body, ok := p.cache.get(key); ok {
		p.write(c, body)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	body, _, err := Transform(f, Options{
		MaxWidth: w,
		Format:   p.Format,
		Quality:  p.Quality,
	})
	if err != nil {
		c.String(http.StatusBadRequest, "imagex: transform")
		return
	}
	p.cache.set(key, body)
	p.write(c, body)
}

func (p *Pipeline) write(c *gin.Context, body []byte) {
	c.Header("Cache-Control", cacheControlVal)
	c.Data(http.StatusOK, contentType(p.Format), body)
}

func contentType(f Format) string {
	switch f {
	case PNG:
		return "image/png"
	case WEBP:
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func (p *Pipeline) resolve(src string) (string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", fmt.Errorf("imagex: missing src")
	}
	lower := strings.ToLower(src)
	if strings.Contains(src, "..") || strings.ContainsAny(src, "\\\x00") ||
		strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(src, "//") {
		return "", fmt.Errorf("imagex: invalid src")
	}
	src = strings.TrimPrefix(src, "/")
	root, err := filepath.Abs(p.Root)
	if err != nil {
		return "", fmt.Errorf("imagex: root")
	}
	full, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(src)))
	if err != nil {
		return "", fmt.Errorf("imagex: invalid src")
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("imagex: invalid src")
	}
	return full, nil
}

// FuncMap returns {{ img src attrs? }} for html/template.
//
//	{{ img "/hero.jpg" }}
//	{{ img "/hero.jpg" "Alt text" }}
//	{{ img "/hero.jpg" (dict "alt" "Hero" "width" 1440 "height" 810 "priority" true) }}
//
// priority omits loading=lazy and sets fetchpriority=high (LCP / hero).
func (p *Pipeline) FuncMap() template.FuncMap {
	p.ready()
	return template.FuncMap{
		"img": p.Img,
	}
}

// Img emits a responsive <img> with srcset against this pipeline.
func (p *Pipeline) Img(src string, attrs ...any) (template.HTML, error) {
	p.ready()
	if strings.TrimSpace(p.Root) == "" {
		return "", fmt.Errorf("imagex: Pipeline.Root is required")
	}
	if _, err := p.resolve(src); err != nil {
		return "", err
	}
	opt := imgAttrs{alt: "", sizes: "(max-width: 768px) 100vw, 1440px"}
	if len(attrs) > 0 {
		switch v := attrs[0].(type) {
		case string:
			opt.alt = v
		case map[string]any:
			opt = parseImgAttrs(v, opt)
		}
	}

	var b strings.Builder
	b.WriteString(`<img src="`)
	b.WriteString(html.EscapeString(p.variantURL(src, p.fallbackWidth(opt.width))))
	b.WriteString(`" srcset="`)
	b.WriteString(html.EscapeString(p.srcset(src)))
	b.WriteString(`" sizes="`)
	b.WriteString(html.EscapeString(opt.sizes))
	b.WriteString(`" alt="`)
	b.WriteString(html.EscapeString(opt.alt))
	b.WriteString(`"`)
	if opt.width > 0 {
		b.WriteString(` width="`)
		b.WriteString(strconv.Itoa(opt.width))
		b.WriteString(`"`)
	}
	if opt.height > 0 {
		b.WriteString(` height="`)
		b.WriteString(strconv.Itoa(opt.height))
		b.WriteString(`"`)
	}
	if opt.class != "" {
		b.WriteString(` class="`)
		b.WriteString(html.EscapeString(opt.class))
		b.WriteString(`"`)
	}
	if opt.priority {
		b.WriteString(` fetchpriority="high" decoding="async"`)
	} else {
		b.WriteString(` loading="lazy" decoding="async"`)
	}
	b.WriteString(`>`)
	return template.HTML(b.String()), nil
}

type imgAttrs struct {
	alt      string
	class    string
	sizes    string
	width    int
	height   int
	priority bool
}

func parseImgAttrs(m map[string]any, opt imgAttrs) imgAttrs {
	if v, ok := m["alt"].(string); ok {
		opt.alt = v
	}
	if v, ok := m["class"].(string); ok {
		opt.class = v
	}
	if v, ok := m["sizes"].(string); ok && v != "" {
		opt.sizes = v
	}
	opt.width = asInt(m["width"])
	opt.height = asInt(m["height"])
	opt.priority = asBool(m["priority"])
	return opt
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func asBool(v any) bool {
	switch n := v.(type) {
	case bool:
		return n
	case string:
		return n == "true" || n == "1"
	default:
		return false
	}
}

func (p *Pipeline) srcset(src string) string {
	parts := make([]string, 0, len(p.widths))
	for _, w := range p.widths {
		parts = append(parts, p.variantURL(src, w)+" "+strconv.Itoa(w)+"w")
	}
	return strings.Join(parts, ", ")
}

func (p *Pipeline) fallbackWidth(hint int) int {
	best := p.widths[len(p.widths)-1]
	if hint <= 0 {
		return best
	}
	chosen := p.widths[0]
	for _, w := range p.widths {
		if w <= hint {
			chosen = w
		}
	}
	return chosen
}

func (p *Pipeline) variantURL(src string, w int) string {
	q := url.Values{}
	q.Set("src", src)
	q.Set("w", strconv.Itoa(w))
	return p.path + "?" + q.Encode()
}

func cacheKey(src string, w int, format Format, quality int) string {
	return src + "|" + strconv.Itoa(w) + "|" + string(format) + "|" + strconv.Itoa(quality)
}

type variantCache struct {
	dir string
	mu  sync.Mutex
	mem map[string][]byte
	ord []string
}

func newVariantCache(dir string) *variantCache {
	return &variantCache{dir: dir, mem: make(map[string][]byte)}
}

func (c *variantCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	if b, ok := c.mem[key]; ok {
		c.mu.Unlock()
		out := make([]byte, len(b))
		copy(out, b)
		return out, true
	}
	c.mu.Unlock()
	if c.dir == "" {
		return nil, false
	}
	data, err := os.ReadFile(c.diskPath(key))
	if err != nil {
		return nil, false
	}
	c.mu.Lock()
	c.remember(key, data)
	c.mu.Unlock()
	return data, true
}

func (c *variantCache) set(key string, body []byte) {
	stored := make([]byte, len(body))
	copy(stored, body)
	c.mu.Lock()
	c.remember(key, stored)
	c.mu.Unlock()
	if c.dir == "" {
		return
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(c.diskPath(key), stored, 0o644)
}

func (c *variantCache) remember(key string, body []byte) {
	if _, ok := c.mem[key]; !ok && len(c.ord) >= memCacheLimit {
		old := c.ord[0]
		c.ord = c.ord[1:]
		delete(c.mem, old)
	}
	if _, ok := c.mem[key]; !ok {
		c.ord = append(c.ord, key)
	}
	c.mem[key] = body
}

func (c *variantCache) diskPath(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".img")
}
