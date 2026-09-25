package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/*.html static
var files embed.FS

// Pages renders the HTML UI and serves its static files.
type Pages struct {
	tmpl *template.Template
}

func New() *Pages {
	return &Pages{
		tmpl: template.Must(template.New("").Funcs(template.FuncMap{
			"markdown": Markdown,
		}).ParseFS(files, "templates/*.html")),
	}
}

func (p *Pages) Static() http.Handler {
	static, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(static)))
}

func (p *Pages) Render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := p.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (p *Pages) Execute(name string, data any) (string, error) {
	var b strings.Builder
	err := p.tmpl.ExecuteTemplate(&b, name, data)
	return b.String(), err
}
