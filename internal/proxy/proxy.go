// Package proxy contains the provider seam for Caddy route reconciliation.
package proxy

import (
	"context"
	"log/slog"
)

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

// LoggingProvider logs every route reconciliation for debugging.
// Wrap the real Caddy provider with it once wired.
type LoggingProvider struct{ Next Provider }

func (l LoggingProvider) Apply(ctx context.Context, route Route) error {
	slog.Info("proxy route apply", "provider_id", route.ProviderID, "path", route.PublicPath, "upstream", route.UpstreamHost, "port", route.UpstreamPort, "enabled", route.Enabled)
	if l.Next == nil {
		return nil
	}
	if err := l.Next.Apply(ctx, route); err != nil {
		slog.Error("proxy route apply failed", "provider_id", route.ProviderID, "error", err)
		return err
	}
	return nil
}

func (l LoggingProvider) Remove(ctx context.Context, providerID string) error {
	slog.Info("proxy route remove", "provider_id", providerID)
	if l.Next == nil {
		return nil
	}
	if err := l.Next.Remove(ctx, providerID); err != nil {
		slog.Error("proxy route remove failed", "provider_id", providerID, "error", err)
		return err
	}
	return nil
}
