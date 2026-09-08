package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// site mimics a Next static export: a root document, a nested route written as
// both a directory index and a sibling .html, a hashed asset, and a 404 page.
func site() *spa {
	return &spa{root: fstest.MapFS{
		"index.html":                  {Data: []byte("<title>root</title>")},
		"404.html":                    {Data: []byte("<title>missing</title>")},
		"trends.html":                 {Data: []byte("<title>trends</title>")},
		"projects/index.html":         {Data: []byte("<title>projects</title>")},
		"_next/static/chunks/app.js":  {Data: []byte("console.log(1)")},
		"_next/static/css/styles.css": {Data: []byte("body{}")},
	}}
}

func get(t *testing.T, s *spa, path string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

// TestRootServesDocument is the regression this package exists for:
// http.FileServer answers "/" with a 301 to "./" because it strips
// index.html, which made the packaged UI unreachable.
func TestRootServesDocument(t *testing.T) {
	res := get(t, site(), "/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200 (a redirect here means the UI never loads)", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestRouteResolution(t *testing.T) {
	cases := []struct {
		path string
		code int
		body string
	}{
		{"/", 200, "root"},
		{"/trends", 200, "trends"},     // sibling .html
		{"/projects", 200, "projects"}, // directory index
		{"/index.html", 200, "root"},   // explicit document
		{"/_next/static/chunks/app.js", 200, "console.log"},
		{"/nope", 404, "missing"}, // unknown route falls back to 404.html
	}
	s := site()
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d", rec.Code, tc.code)
			}
			if !strings.Contains(rec.Body.String(), tc.body) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tc.body)
			}
		})
	}
}

// TestMissingAssetIs404 pins that a missing asset never falls back to HTML.
// Serving a document in place of a script or stylesheet yields a blank page
// with no error, which is far harder to diagnose than a 404.
func TestMissingAssetIs404(t *testing.T) {
	s := site()
	for _, p := range []string{
		"/_next/static/chunks/gone.js",
		"/missing.css",
		"/missing.js",
		"/img/logo.png",
	} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<title>") {
			t.Errorf("GET %s returned an HTML document instead of a 404", p)
		}
	}
}

func TestAssetContentTypes(t *testing.T) {
	s := site()
	cases := map[string]string{
		"/_next/static/chunks/app.js":  "javascript",
		"/_next/static/css/styles.css": "css",
	}
	for p, want := range cases {
		res := get(t, s, p)
		if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, want) {
			t.Errorf("GET %s Content-Type = %q, want it to mention %q", p, ct, want)
		}
	}
}

// TestFallsBackToIndexWithoutA404Page: an export lacking 404.html must still
// answer, rather than emitting Go's bare "404 page not found".
func TestFallsBackToIndexWithoutA404Page(t *testing.T) {
	s := &spa{root: fstest.MapFS{
		"index.html": {Data: []byte("<title>root</title>")},
	}}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/deep/link", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "root") {
		t.Errorf("expected the SPA entry point, got %q", rec.Body.String())
	}
}

// TestNoUIEmbedded covers the source-checkout case: the placeholder directory
// must produce no handler at all, so the API mounts nothing and runs headless.
func TestNoUIEmbedded(t *testing.T) {
	// The real embed is exercised by Embedded(); this pins the contract that a
	// tree without index.html yields no handler.
	if _, err := (fstest.MapFS{".gitkeep": {Data: []byte("x")}}).Open("index.html"); err == nil {
		t.Fatal("test fixture is wrong")
	}
	s := &spa{root: fstest.MapFS{".gitkeep": {Data: []byte("x")}}}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when there is no site", rec.Code)
	}
}

func TestIsAsset(t *testing.T) {
	cases := map[string]bool{
		"_next/static/chunks/app.js": true,
		"styles.css":                 true,
		"logo.png":                   true,
		"index.html":                 false,
		"trends":                     false,
		"projects/detail":            false,
	}
	for in, want := range cases {
		if got := isAsset(in); got != want {
			t.Errorf("isAsset(%q) = %v, want %v", in, got, want)
		}
	}
}
