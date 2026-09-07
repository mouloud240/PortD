// Package proxy contains the provider seam for Caddy route reconciliation.
package proxy

import "context"

// Route describes the desired provider configuration for one project.
type Route struct {
	ProviderID   string
	PublicPath   string
	UpstreamHost string
	UpstreamPort int64
	Enabled      bool
}

// Provider applies routes idempotently to a reverse proxy.
type Provider interface {
	Apply(context.Context, Route) error
	Remove(context.Context, string) error
}

// FakeProvider records route operations for unit tests.
type FakeProvider struct {
	Applied []Route
	Removed []string
	Err     error
}

func (f *FakeProvider) Apply(_ context.Context, route Route) error {
	if f.Err != nil {
		return f.Err
	}
	f.Applied = append(f.Applied, route)
	return nil
}

func (f *FakeProvider) Remove(_ context.Context, providerID string) error {
	if f.Err != nil {
		return f.Err
	}
	f.Removed = append(f.Removed, providerID)
	return nil
}
