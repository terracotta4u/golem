package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
)

// Extensions serves the extension pages.
// Start and stop run after install and before remove. Nil skips that step.
type Extensions struct {
	pages *Pages
	start func(name string) error
	stop  func(name string) error
}

func NewExtensions(pages *Pages, start, stop func(name string) error) *Extensions {
	return &Extensions{pages: pages, start: start, stop: stop}
}

func (h *Extensions) List(w http.ResponseWriter, r *http.Request) {
	root, err := conf.ExtensionsDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	installed, err := extension.List(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type row struct {
		Name    string
		Version string
		Source  string
	}
	var list []row
	for _, p := range installed {
		list = append(list, row{
			Name:    p.Name,
			Version: p.Version,
			Source:  cfg.Extensions[p.Name].Source,
		})
	}
	h.pages.Render(w, "extensions", map[string]any{
		"Title":      "Extensions",
		"Extensions": list,
		"PageCSS":    "settings.css",
	})
}

func (h *Extensions) Detail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || name != filepath.Base(name) {
		http.NotFound(w, r)
		return
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, err := extension.Load(filepath.Join(root, name))
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	origin := cfg.Extensions[name]
	h.pages.Render(w, "extension", map[string]any{
		"Title":       p.Name,
		"Name":        p.Name,
		"Version":     p.Version,
		"Description": p.Description,
		"Source":      origin.Source,
		"Ref":         origin.Ref,
		"Revision":    origin.Revision,
		"PageCSS":     "settings.css",
	})
}

func (h *Extensions) Remove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if h.stop != nil {
		if err := h.stop(name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := extension.Remove(root, name); err != nil {
		if strings.Contains(err.Error(), "not installed") || strings.Contains(err.Error(), "invalid extension name") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conf.RemoveExtension(&cfg, name)
	if err := conf.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/extensions", http.StatusSeeOther)
}

func (h *Extensions) Add(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	if from != "archive" {
		from = "url"
	}
	h.pages.Render(w, "extension-add", map[string]any{
		"Title":   "Add extension",
		"From":    from,
		"PageCSS": "settings.css",
	})
}

func (h *Extensions) AddURL(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	src := strings.TrimSpace(r.FormValue("url"))
	if src == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	h.install(w, r, src, strings.TrimSpace(r.FormValue("ref")), "")
}

func (h *Extensions) AddArchive(w http.ResponseWriter, r *http.Request) {
	const maxArchive = 32 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxArchive)
	if err := r.ParseMultipartForm(maxArchive); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("archive")
	if err != nil {
		http.Error(w, "archive is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if !strings.EqualFold(filepath.Ext(hdr.Filename), ".zip") {
		http.Error(w, "archive must be a zip file", http.StatusBadRequest)
		return
	}

	tmp, err := os.CreateTemp("", "golem-ext-*.zip")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmp.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.install(w, r, tmpName, "", hdr.Filename)
}

func (h *Extensions) install(w http.ResponseWriter, r *http.Request, src, ref, originSource string) {
	root, err := conf.ExtensionsDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, err := extension.Install(src, root, extension.Options{Ref: ref})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if originSource == "" {
		originSource = p.Origin.Source
	}
	conf.SetExtensionOrigin(&cfg, p.Name, originSource, p.Origin.Ref, p.Origin.Revision)
	if err := conf.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.start != nil {
		if err := h.start(p.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/settings/extensions", http.StatusSeeOther)
}
