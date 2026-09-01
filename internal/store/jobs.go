package store

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

// Job mirrors the frontend SchedulerProfile in web/src/types/domain.ts. Status
// is "active" | "paused"; the UI can currently only ever produce "active".
type Job struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Keywords  []string  `json:"keywords"`
	Cron      string    `json:"cron"`
	CreatedAt time.Time `json:"createdAt"`
	Status    string    `json:"status"`
}

func (s *Store) Jobs() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readJobs()
}

func (s *Store) readJobs() ([]Job, error) {
	b, err := os.ReadFile(s.jobsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var jobs []Job
	if err := json.Unmarshal(b, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// AddJob appends and persists, assigning the id the UI displays.
func (s *Store) AddJob(j Job) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.readJobs()
	if err != nil {
		return Job{}, err
	}
	if j.ID == "" {
		j.ID = "job-" + strconv.Itoa(len(jobs)+1)
	}
	if j.Status == "" {
		j.Status = "active"
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now().UTC()
	}
	jobs = append(jobs, j)
	return j, s.writeJobs(jobs)
}
