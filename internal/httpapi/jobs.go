package httpapi

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/harvest"
)

// cronField matches one field of a five-field expression. Only "*", a plain
// integer, and a comma-separated list are supported, which covers everything
// the scheduler UI can produce (daily, weekly and monthly at a chosen time).
func cronField(spec string, v int) bool {
	if spec == "*" || spec == "" {
		return true
	}
	for _, part := range strings.Split(spec, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n == v {
			return true
		}
	}
	return false
}

// cronMatches reports whether "min hour dom month dow" fires at t.
func cronMatches(expr string, t time.Time) bool {
	f := strings.Fields(expr)
	if len(f) != 5 {
		return false
	}
	return cronField(f[0], t.Minute()) &&
		cronField(f[1], t.Hour()) &&
		cronField(f[2], t.Day()) &&
		cronField(f[3], int(t.Month())) &&
		cronField(f[4], int(t.Weekday()))
}

// RunScheduler fires registered jobs on their cron. Without it the scheduler is
// write-only: the UI can register a rule that never runs.
//
// ponytail: one minute-granularity tick over all jobs, no persistence of the
// last fire time — a job that was due while the process was down is simply
// missed. Persist last-run per job if catch-up ever matters.
func (s *Server) RunScheduler(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	last := time.Now().Truncate(time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			now = now.Truncate(time.Minute)
			if now.Equal(last) {
				continue
			}
			last = now
			jobs, err := s.store.Jobs()
			if err != nil {
				continue
			}
			for _, j := range jobs {
				if j.Status != "active" || !cronMatches(j.Cron, now) {
					continue
				}
				log.Printf("scheduler: firing job %s (%s)", j.ID, j.Name)
				task := s.hub.New(newTaskID(), "harvest")
				go s.runHarvest(ctx, task, "job-"+j.ID+"-"+now.Format("20060102-1504"),
					fmt.Sprintf("%s (scheduled)", j.Name), harvest.Query{
						Categories: s.cfg.Categories,
						Keywords:   j.Keywords,
						From:       now.Year() - 7,
						To:         now.Year(),
						Max:        s.cfg.HarvestMax,
					})
			}
		}
	}
}
