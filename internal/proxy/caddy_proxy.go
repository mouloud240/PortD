package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// assetPrefixes are the root-absolute static paths apps emit (they assume
// they own "/"). Each gets a Referer-scoped route per project.
var assetPrefixes = []string{"css", "js", "assets"}

var viteDevPaths = []string{
	"/@vite/*",
	"/@react-refresh",
	"/@id/*",
	"/@fs/*",
	"/src/*",
	"/node_modules/*", // covers /node_modules/.vite/* and /node_modules/vite/dist/client/env.mjs
}
const healthTTL = 30 * time.Second

const probeTimeout = 2 * time.Second

type CaddyProvider struct {
	baseURL string
	http    *http.Client

	mu        sync.Mutex
	isHealthy bool
	checkedAt time.Time
	probed    bool
	ttl       time.Duration
}

func NewCaddyProvider(address string) *CaddyProvider {
	address = strings.TrimSpace(address)
	if address == "" {
		address = "http://localhost:2019"
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	return &CaddyProvider{
		baseURL: strings.TrimRight(address, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
		ttl:     healthTTL,
	}
}

func (c *CaddyProvider) IsHealthy(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.probed && time.Since(c.checkedAt) < c.ttl {
		return c.isHealthy
	}
	c.isHealthy = c.probe(ctx)
	c.checkedAt = time.Now()
	c.probed = true
	return c.isHealthy
}

func (c *CaddyProvider) probe(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/config/", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return resp.StatusCode < 500
}

// adminRoute mirrors Caddy's native route JSON for the shapes we post.
type adminRoute struct {
	ID     string       `json:"@id,omitempty"`
	Match  []adminMatch `json:"match,omitempty"`
	Handle []adminOp    `json:"handle,omitempty"`
}

type adminMatch struct {
	Path   []string            `json:"path,omitempty"`
	Host   []string            `json:"host,omitempty"`
	Header map[string][]string `json:"header,omitempty"`
}

type adminOp struct {
	Handler         string          `json:"handler"`
	Upstreams       []adminUpstream `json:"upstreams,omitempty"`
	StripPathPrefix string          `json:"strip_path_prefix,omitempty"`
}

type adminUpstream struct {
	Dial string `json:"dial"`
}

func proxyOp(host string, port int64) adminOp {
	if host == "" {
		host = "localhost"
	}
	return adminOp{Handler: "reverse_proxy", Upstreams: []adminUpstream{{Dial: fmt.Sprintf("%s:%d", host, port)}}}
}

// Apply posts 5 routes: main (/slug, /slug/*) plus one Referer-scoped
// asset route per prefix (/css|/js|/assets/* AND Referer ~ /slug),
// plus a host route (Host slug.*) serving the app at domain root as a
// fallback that needs no base path.
func (c *CaddyProvider) Apply(ctx context.Context, route Route) error {
	if route.ProviderID == "" {
		return errors.New("proxy: missing provider route id")
	}
	if route.UpstreamPort < 1 || route.UpstreamPort > 65535 {
		return fmt.Errorf("proxy: invalid upstream port %d", route.UpstreamPort)
	}
	base := "/" + strings.Trim(route.PublicPath, "/")
	slug := strings.Trim(base, "/")
	if slug == "" {
		return errors.New("proxy: missing public path")
	}
	if !c.IsHealthy(ctx) {
		slog.Warn("proxy apply skipped: caddy unavailable", "provider_id", route.ProviderID)
		return nil
	}

		routes := []adminRoute{{
		ID: route.ProviderID,
		Match: []adminMatch{{
			Path: append([]string{base, base + "/*"}, viteDevPaths...),
		}},
		Handle: []adminOp{
			proxyOp(route.UpstreamHost, route.UpstreamPort),
		},
	}}
	for _, prefix := range assetPrefixes {
		routes = append(routes, adminRoute{
			ID: route.ProviderID + "-" + prefix,
			Match: []adminMatch{{
				Path: []string{"/" + prefix + "/*"},
				// Same set = AND; both values = OR. Slash-delimited so
				// slug "app" never matches Referer ".../myapp/...".
				Header: map[string][]string{"Referer": {"*/" + slug + "/*", "*/" + slug}},
			}},
			Handle: []adminOp{
				{Handler: "rewrite", StripPathPrefix: "/" + prefix},
				proxyOp(route.UpstreamHost, route.UpstreamPort),
			},
		})
	}
		routes = append(routes, adminRoute{
		ID: route.ProviderID + "-host",
		Match: []adminMatch{{
			Host: []string{slug + ".*", slug + ".*.*", slug + ".*.*.*", slug + ".*.*.*.*",slug +".*.*.*.*.nip.io"},
		}},
		Handle: []adminOp{
			proxyOp(route.UpstreamHost, route.UpstreamPort),
		},
	})

	for _, r := range routes {
		if err := c.postRoute(ctx, r); err != nil {
			return fmt.Errorf("proxy: apply route %s: %w", r.ID, err)
		}
	}
	slog.Info("proxy route applied", "provider_id", route.ProviderID, "path", base, "routes", len(routes))
	return nil
}

// Remove deletes the main route plus its asset and host routes by @id.
// Missing IDs (404) are already-gone, not errors.
func (c *CaddyProvider) Remove(ctx context.Context, providerID string) error {
	if providerID == "" {
		return errors.New("proxy: missing provider route id")
	}
	if !c.IsHealthy(ctx) {
		slog.Warn("proxy remove skipped: caddy unavailable", "provider_id", providerID)
		return nil
	}
	ids := []string{providerID}
	for _, prefix := range assetPrefixes {
		ids = append(ids, providerID+"-"+prefix)
	}
	ids = append(ids, providerID+"-host")
	var errs []error
	for _, id := range ids {
		if err := c.deleteByID(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("remove route %s: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	slog.Info("proxy route removed", "provider_id", providerID)
	return nil
}

func (c *CaddyProvider) postRoute(ctx context.Context, route adminRoute) error {
	body, err := json.Marshal(route)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/config/apps/http/servers/srv0/routes", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *CaddyProvider) deleteByID(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/id/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.do(req); err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	return nil
}

var errNotFound = errors.New("caddy: not found")

func (c *CaddyProvider) do(req *http.Request) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("caddy request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("caddy status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}
