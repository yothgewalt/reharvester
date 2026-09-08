// Package store owns the on-disk layout. Everything the pipeline produces lands
// under a project directory as an inspectable file, which is what lets any stage
// be re-run or replaced without re-running the ones before it.
//
//	<root>/jobs.json
//	<root>/projects/<id>/meta.json        project record served to the UI
//	<root>/projects/<id>/papers.jsonl     source of truth, one Paper per line
//	<root>/projects/<id>/graph.json       backbone + communities + pagerank
//	<root>/projects/<id>/analytics.json   trends + gap pairs
//	<root>/projects/<id>/embeddings.bin   dense vectors, float32 LE, row-major
//
// The lexical indexes (inverted, TF-IDF, BM25) are deliberately NOT persisted:
// rebuilding them from papers.jsonl costs a few seconds at startup, which is
// cheaper than keeping a serialised copy correct. Embeddings are persisted
// because encoding is the one stage measured in minutes.
package store

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

type Store struct {
	root string
	mu   sync.Mutex // serialises jobs.json read-modify-write
}

func Open(root string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Root() string { return s.root }

// Meta is the project record the UI lists. Status mirrors the frontend's
// Project.status union exactly: "crawling" | "complete" | "failed".
type Meta struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Query        string    `json:"query"`
	CreatedAt    time.Time `json:"createdAt"`
	Status       string    `json:"status"`
	DocsIngested int       `json:"docsIngested"`
}

type Project struct {
	ID  string
	dir string
}

func (s *Store) Project(id string) (*Project, error) {
	dir := filepath.Join(s.root, "projects", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Project{ID: id, dir: dir}, nil
}

func (s *Store) Projects() ([]Meta, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "projects"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Meta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := s.Project(e.Name())
		if err != nil {
			continue
		}
		var m Meta
		if err := p.LoadJSON("meta.json", &m); err == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (p *Project) Path(name string) string { return filepath.Join(p.dir, name) }

// SaveJSON writes atomically: a half-written artefact that a later stage reads
// as valid is the failure mode worth spending a rename on.
func (p *Project) SaveJSON(name string, v any) error {
	tmp := p.Path(name + ".tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p.Path(name))
}

func (p *Project) LoadJSON(name string, v any) error {
	b, err := os.ReadFile(p.Path(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (p *Project) Exists(name string) bool {
	_, err := os.Stat(p.Path(name))
	return err == nil
}

func (p *Project) WritePapers(ps []paper.Paper) error {
	tmp := p.Path("papers.jsonl.tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	enc := json.NewEncoder(w)
	for i := range ps {
		if err := enc.Encode(&ps[i]); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p.Path("papers.jsonl"))
}

func (p *Project) ReadPapers() ([]paper.Paper, error) {
	f, err := os.Open(p.Path("papers.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []paper.Paper
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<22)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var pp paper.Paper
		if err := json.Unmarshal(line, &pp); err != nil {
			return nil, fmt.Errorf("papers.jsonl: %w", err)
		}
		out = append(out, pp)
	}
	return out, sc.Err()
}

// SaveVectors stores a row-major float32 matrix: an int32 row count, an int32
// dimension, then rows*dim little-endian float32s.
func (p *Project) SaveVectors(name string, vecs [][]float32) error {
	if len(vecs) == 0 {
		return nil
	}
	dim := len(vecs[0])
	tmp := p.Path(name + ".tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	hdr := []int32{int32(len(vecs)), int32(dim)}
	if err := binary.Write(w, binary.LittleEndian, hdr); err != nil {
		f.Close()
		return err
	}
	buf := make([]byte, 4*dim)
	for _, v := range vecs {
		if len(v) != dim {
			f.Close()
			return fmt.Errorf("ragged vectors: got %d want %d", len(v), dim)
		}
		for i, x := range v {
			binary.LittleEndian.PutUint32(buf[4*i:], math.Float32bits(x))
		}
		if _, err := w.Write(buf); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p.Path(name))
}

func (p *Project) LoadVectors(name string) ([][]float32, error) {
	f, err := os.Open(p.Path(name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var hdr [2]int32
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}
	rows, dim := int(hdr[0]), int(hdr[1])
	if rows < 0 || dim <= 0 {
		return nil, fmt.Errorf("%s: bad header %d x %d", name, rows, dim)
	}
	flat := make([]float32, rows*dim)
	buf := make([]byte, 4*dim)
	for i := range rows {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		row := flat[i*dim : (i+1)*dim]
		for j := range dim {
			row[j] = math.Float32frombits(binary.LittleEndian.Uint32(buf[4*j:]))
		}
	}
	out := make([][]float32, rows)
	for i := range rows {
		out[i] = flat[i*dim : (i+1)*dim]
	}
	return out, nil
}
