package proxy

import (
	"context"
	"testing"
)

func TestFakeProviderRecordsIdempotentOperations(t *testing.T) {
	t.Parallel()

	provider := &FakeProvider{}
	route := Route{ProviderID: "demo", PublicPath: "/demo", UpstreamHost: "127.0.0.1", UpstreamPort: 3000, Enabled: true}
	if err := provider.Apply(context.Background(), route); err != nil {
		t.Fatal(err)
	}
	if err := provider.Remove(context.Background(), route.ProviderID); err != nil {
		t.Fatal(err)
	}
	if len(provider.Applied) != 1 || len(provider.Removed) != 1 {
		t.Fatalf("applied = %+v, removed = %+v", provider.Applied, provider.Removed)
	}
}
