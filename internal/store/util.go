package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *Store) jobsPath() string { return filepath.Join(s.root, "jobs.json") }

func (s *Store) writeJobs(jobs []Job) error {
	b, err := json.MarshalIndent(jobs, "", " ")
	if err != nil {
		return err
	}
	tmp := s.jobsPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.jobsPath())
}
