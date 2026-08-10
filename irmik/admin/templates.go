package admin

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/*.html
var templateFS embed.FS

// TemplatesFS is the embedded admin CRUD snippet filesystem
// (list, form, confirm_delete, flash).
func TemplatesFS() fs.FS {
	sub, err := fs.Sub(templateFS, "templates")
	if err != nil {
		return templateFS
	}
	return sub
}

// ParseTemplates parses embedded admin snippets into a template set.
// Extra funcs are merged (may be nil). Template names match file basenames
// (e.g. "list.html", "form.html", "confirm_delete.html", "flash.html").
func ParseTemplates(funcs template.FuncMap) (*template.Template, error) {
	t := template.New("admin")
	if funcs != nil {
		t = t.Funcs(funcs)
	}
	return t.ParseFS(templateFS, "templates/*.html")
}

// MustParseTemplates is ParseTemplates that panics on error.
func MustParseTemplates(funcs template.FuncMap) *template.Template {
	t, err := ParseTemplates(funcs)
	if err != nil {
		panic(err)
	}
	return t
}
