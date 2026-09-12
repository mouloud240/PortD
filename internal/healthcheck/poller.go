package healthcheck

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	db "github.com/portd/internal/db/generated"
)

// Store provides the persisted healthchecks and project state update needed by
// the poller.
type Store interface {
	ListAllProjectHealthchecks(context.Context) ([]db.ProjectHealthcheck, error)
	UpdateProjectHealthcheckState(context.Context, db.UpdateProjectHealthcheckStateParams) (db.Project, error)
}

// Poller periodically evaluates every configured project healthcheck.
type Poller struct {
	store    Store
	client   Client
	interval time.Duration
	workers  int
	onError  func(error)
}

type checkJob struct {
	check  db.ProjectHealthcheck
	result chan<- checkResult
}

type checkResult struct {
	check db.ProjectHealthcheck
	err   error
}

// PollerOption customizes a Poller.
type PollerOption func(*Poller) error

// WithInterval sets the polling interval.
func WithInterval(interval time.Duration) PollerOption {
	return func(p *Poller) error {
		if interval <= 0 {
			return errors.New("healthcheck: interval must be positive")
		}
		p.interval = interval
		return nil
	}
}

// WithWorkers sets the maximum number of concurrent healthcheck requests.
func WithWorkers(workers int) PollerOption {
	return func(p *Poller) error {
		if workers < 1 {
			return errors.New("healthcheck: workers must be positive")
		}
		p.workers = workers
		return nil
	}
}

// WithErrorHandler handles errors from background polling cycles.
func WithErrorHandler(handler func(error)) PollerOption {
	return func(p *Poller) error {
		if handler == nil {
			return errors.New("healthcheck: error handler is nil")
		}
		p.onError = handler
		return nil
	}
}

// NewPoller constructs a healthcheck poller with a one-minute cadence and four
// concurrent workers by default.
func NewPoller(store Store, client Client, options ...PollerOption) (*Poller, error) {
	if store == nil {
		return nil, errors.New("healthcheck: store is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	poller := &Poller{
		store:    store,
		client:   client,
		interval: time.Minute,
		workers:  4,
		onError:  func(error) {},
	}
	for _, option := range options {
		if err := option(poller); err != nil {
			return nil, err
		}
	}
	return poller, nil
}

// Run performs an initial cycle and continues until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	jobs := make(chan checkJob)
	var workers sync.WaitGroup
	workers.Add(p.workers)
	for range p.workers {
		go p.worker(ctx, jobs, &workers)
	}
	defer func() {
		close(jobs)
		workers.Wait()
	}()

	if err := p.runCycle(ctx, jobs); err != nil && ctx.Err() == nil {
		p.onError(err)
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.runCycle(ctx, jobs); err != nil && ctx.Err() == nil {
				p.onError(err)
			}
		}
	}
}

// RunCycle evaluates one snapshot of the configured checks. Calls are
// synchronous so a ticker event arriving during a cycle is skipped rather than
// queued as duplicate work.
func (p *Poller) RunCycle(ctx context.Context) error {
	jobs := make(chan checkJob)
	var workers sync.WaitGroup
	workers.Add(p.workers)
	for range p.workers {
		go p.worker(ctx, jobs, &workers)
	}
	defer func() {
		close(jobs)
		workers.Wait()
	}()
	return p.runCycle(ctx, jobs)
}

func (p *Poller) runCycle(ctx context.Context, jobs chan<- checkJob) error {
	checks, err := p.store.ListAllProjectHealthchecks(ctx)
	if err != nil {
		return err
	}
	if len(checks) == 0 {
		return nil
	}

	results := make(chan checkResult, len(checks))
	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- checkJob{check: check, result: results}:
		}
	}

	healthy := make(map[string]bool)
	seen := make(map[string]bool)
	for range checks {
		item := <-results
		if !seen[item.check.ProjectID] {
			healthy[item.check.ProjectID] = true
			seen[item.check.ProjectID] = true
		}
		if item.err != nil {
			healthy[item.check.ProjectID] = false
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var updateErr error
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for projectID, isHealthy := range healthy {
		live := int64(0)
		lifecycle := "stopped"
		if isHealthy {
			live = 1
			lifecycle = "running"
		}
		if _, err := p.store.UpdateProjectHealthcheckState(ctx, db.UpdateProjectHealthcheckStateParams{
			IsLive:          live,
			LifecycleStatus: lifecycle,
			UpdatedAt:       now,
			ID:              projectID,
		}); err != nil {
			updateErr = errors.Join(updateErr, err)
		}
	}
	return updateErr
}

func (p *Poller) worker(ctx context.Context, jobs <-chan checkJob, workers *sync.WaitGroup) {
	defer workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}
			outcome, err := New(p.client).Check(ctx, job.check.Endpoint, int(job.check.ExpectedStatus))
			if err == nil && !outcome.Healthy {
				err = errors.New("healthcheck: endpoint returned unexpected status")
			}
			select {
			case job.result <- checkResult{check: job.check, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}
}
