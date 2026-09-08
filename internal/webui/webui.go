// Package webui carries the exported web UI inside the binary, so a packaged
// install serves the interface from the same process as the API and needs no
// Node runtime. The release script copies web/dist here before building; a
// source checkout has only a placeholder, and the API then runs headless.
package webui

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Embedded reports whether a real UI was built into this binary. Callers use it
// to decide between "open your browser" and "API only"; it is false for every
// `go build` that did not run the release script first.
func Embedded() bool {
	_, err := content()
	return err == nil
}

// Handler serves the exported UI, or nil when none was embedded. A nil handler
// is the headless configuration, not an error — mount it only when non-nil.
func Handler() http.Handler {
	root, err := content()
	if err != nil {
		return nil
	}
	return &spa{root: root}
}

func content() (fs.FS, error) {
	root, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return nil, err
	}
	return root, nil
}

// spa resolves a request the way a static host does: exact file, then
// directory index, then the `.html` sibling Next writes for each route. Only
// document requests fall back to index.html — a missing asset must stay a 404,
// because serving HTML in place of a stylesheet produces a blank page with no
// error rather than a visible failure.
//
// Content is served directly rather than through http.FileServer, which
// redirects any path ending in /index.html to ./ and would turn a request for
// the root into a 301 loop.
type spa struct {
	root fs.FS
}

func (s *spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}
	if resolved, ok := s.resolve(p); ok {
		s.serveFile(w, r, resolved, 0)
		return
	}
	if isAsset(p) {
		http.NotFound(w, r)
		return
	}
	s.serveHTML(w, r, "404.html", http.StatusNotFound)
}

// serveFile writes one embedded file. A non-zero code forces that status,
// which the 404 document needs; otherwise http.ServeContent decides, so
// conditional requests and ranges keep working.
func (s *spa) serveFile(w http.ResponseWriter, r *http.Request, name string, code int) {
	f, err := s.root.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		b, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		rs = bytes.NewReader(b)
	}
	if code != 0 {
		if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(code)
		_, _ = io.Copy(w, rs)
		return
	}
	http.ServeContent(w, r, path.Base(name), info.ModTime(), rs)
}

func (s *spa) resolve(p string) (string, bool) {
	if st, err := fs.Stat(s.root, p); err == nil && !st.IsDir() {
		return p, true
	}
	for _, cand := range []string{p + "/index.html", p + ".html"} {
		if st, err := fs.Stat(s.root, cand); err == nil && !st.IsDir() {
			return cand, true
		}
	}
	return "", false
}

// serveHTML sends a document with an explicit status, falling back to the SPA
// entry point when the export carries no 404 page.
func (s *spa) serveHTML(w http.ResponseWriter, r *http.Request, name string, code int) {
	if _, err := fs.Stat(s.root, name); err != nil {
		if name != "index.html" {
			s.serveHTML(w, r, "index.html", code)
			return
		}
		http.NotFound(w, r)
		return
	}
	s.serveFile(w, r, name, code)
}

// isAsset marks paths that must 404 rather than fall back to HTML.
func isAsset(p string) bool {
	if strings.HasPrefix(p, "_next/") {
		return true
	}
	i := strings.LastIndexByte(p, '.')
	if i < 0 {
		return false
	}
	switch strings.ToLower(p[i:]) {
	case ".html":
		return false
	default:
		return !strings.Contains(p[i:], "/")
	}
}
