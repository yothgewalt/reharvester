package harvest

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

// Snapshot harvests from Cornell's arXiv metadata dump on Kaggle
// (arxiv-metadata-oai-snapshot.json, one JSON record per line), with no network
// and no rate limit. Path may be the .json or the .zip Kaggle downloads.
//
// The whole file is scanned on every harvest — a few minutes for the full dump
// — and selection follows localSelect. Categories filter exactly as on arXiv.
type Snapshot struct {
	Path string
}

const snapshotReportEvery = 250_000

type snapshotRecord struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Abstract      string     `json:"abstract"`
	Categories    string     `json:"categories"`
	AuthorsParsed [][]string `json:"authors_parsed"`
	Versions      []struct {
		Created string `json:"created"`
	} `json:"versions"`
}

func (s *Snapshot) Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error) {
	if prog == nil {
		prog = func(int, int, string) {}
	}
	r, err := openSnapshot(s.Path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	sel := newLocalSelect(q)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	lines := 0
	for sc.Scan() {
		lines++
		if lines%snapshotReportEvery == 0 {
			if err := ctx.Err(); err != nil {
				return sel.result(), err
			}
			prog(0, sel.Matched, fmt.Sprintf("scanned %d records, %d match", lines, sel.Matched))
		}
		var rec snapshotRecord
		if json.Unmarshal(sc.Bytes(), &rec) != nil {
			continue
		}
		if p, ok := rec.toPaper(); ok {
			sel.offer(p)
		}
	}
	out := sel.result()
	if err := sc.Err(); err != nil {
		return out, fmt.Errorf("reading %s: %w", s.Path, err)
	}
	prog(len(out), sel.Matched, fmt.Sprintf("scanned %d records; kept %d of %d matching", lines, len(out), sel.Matched))
	return out, nil
}

func (r snapshotRecord) toPaper() (paper.Paper, bool) {
	var published time.Time
	if len(r.Versions) > 0 {
		published, _ = time.Parse("Mon, 2 Jan 2006 15:04:05 MST", r.Versions[0].Created)
	}
	if published.IsZero() {
		published = arxivIDDate(r.ID)
	}
	authors := make([]string, 0, len(r.AuthorsParsed))
	for _, a := range r.AuthorsParsed {
		// [last, first, suffix]
		if len(a) >= 2 {
			a[0], a[1] = a[1], a[0]
		}
		authors = append(authors, strings.Join(a, " "))
	}
	return newPaper(paper.StripVersion(r.ID), r.Title, r.Abstract, authors, strings.Fields(r.Categories), published, true)
}

// openSnapshot opens the dump, reading the first .json entry of a .zip in
// place so the multi-gigabyte file need not be extracted.
func openSnapshot(path string) (io.ReadCloser, error) {
	// The TUI passes the path as typed, with no shell to expand "~".
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	}
	if !strings.HasSuffix(strings.ToLower(path), ".zip") {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening the arXiv snapshot: %w", err)
		}
		return f, nil
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("opening the arXiv snapshot: %w", err)
	}
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".json") {
			rc, err := f.Open()
			if err != nil {
				zr.Close()
				return nil, err
			}
			return zipEntry{rc, zr}, nil
		}
	}
	zr.Close()
	return nil, fmt.Errorf("%s holds no .json file", path)
}

type zipEntry struct {
	io.ReadCloser
	zr *zip.ReadCloser
}

func (z zipEntry) Close() error {
	z.ReadCloser.Close()
	return z.zr.Close()
}
