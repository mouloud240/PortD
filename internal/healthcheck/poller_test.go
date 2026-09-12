package healthcheck

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	db "github.com/portd/internal/db/generated"
)

type pollerStore struct {
	checks  []db.ProjectHealthcheck
	updates []db.UpdateProjectHealthcheckStateParams
}

func (s *pollerStore) ListAllProjectHealthchecks(context.Context) ([]db.ProjectHealthcheck, error) {
	return s.checks, nil
}

func (s *pollerStore) UpdateProjectHealthcheckState(_ context.Context, params db.UpdateProjectHealthcheckStateParams) (db.Project, error) {
	s.updates = append(s.updates, params)
	return db.Project{}, nil
}

type pollerClient struct {
	mu       sync.Mutex
	requests int
	active   int
	max      int
	statuses map[string]int
}

func (c *pollerClient) Do(request *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.requests++
	c.active++
	if c.active > c.max {
		c.max = c.active
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
	}()
	status := c.statuses[request.URL.String()]
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestPollerRunCycleAggregatesAllChecks(t *testing.T) {
	t.Parallel()

	store := &pollerStore{checks: []db.ProjectHealthcheck{
		{ID: "a", ProjectID: "p1", Endpoint: "http://one.test/health", ExpectedStatus: 200},
		{ID: "b", ProjectID: "p1", Endpoint: "http://two.test/health", ExpectedStatus: 204},
		{ID: "c", ProjectID: "p2", Endpoint: "http://three.test/health", ExpectedStatus: 200},
	}}
	client := &pollerClient{statuses: map[string]int{
		"http://one.test/health":   200,
		"http://two.test/health":   200,
		"http://three.test/health": 200,
	}}
	poller, err := NewPoller(store, client, WithWorkers(2))
	if err != nil {
		t.Fatal(err)
	}
	if err := poller.RunCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(store.updates))
	}
	for _, update := range store.updates {
		if update.ID == "p1" && (update.IsLive != 0 || update.LifecycleStatus != "stopped") {
			t.Fatalf("p1 update = %+v, want stopped", update)
		}
		if update.ID == "p2" && (update.IsLive != 1 || update.LifecycleStatus != "running") {
			t.Fatalf("p2 update = %+v, want running", update)
		}
	}
	if client.max > 2 {
		t.Fatalf("max concurrent requests = %d, want <= 2", client.max)
	}
}

func TestPollerRunCycleStopsOnCancellation(t *testing.T) {
	t.Parallel()

	store := &pollerStore{checks: []db.ProjectHealthcheck{
		{ID: "a", ProjectID: "p1", Endpoint: "http://one.test/health", ExpectedStatus: 200},
	}}
	poller, err := NewPoller(store, &pollerClient{statuses: map[string]int{"http://one.test/health": 200}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := poller.RunCycle(ctx); err != context.Canceled {
		t.Fatalf("RunCycle error = %v, want %v", err, context.Canceled)
	}
}

func TestPollerOptionsValidate(t *testing.T) {
	t.Parallel()

	store := &pollerStore{}
	for _, test := range []struct {
		name string
		opt  PollerOption
	}{
		{"zero interval", WithInterval(0)},
		{"zero workers", WithWorkers(0)},
		{"nil error handler", WithErrorHandler(nil)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewPoller(store, nil, test.opt); err == nil {
				t.Fatal("NewPoller returned nil error")
			}
		})
	}
}
