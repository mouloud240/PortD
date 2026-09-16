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
	"time"
)

// assetPrefixes are the root-absolute static paths apps emit (they assume
// they own "/"). Each gets a Referer-scoped route per project.
var assetPrefixes = []string{"css", "js", "assets"}

// CaddyProvider talks to the Caddy admin API with raw calls: the
// go.destructure.dev/caddy lib has no header-matcher or rewrite types,
// and its DeleteConfig cannot hit DELETE /id/<id>.
type CaddyProvider struct {
	baseURL string
	http    *http.Client
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
	}
}

// adminRoute mirrors Caddy's native route JSON for the shapes we post.
type adminRoute struct {
	ID     string       `json:"@id,omitempty"`
	Match  []adminMatch `json:"match,omitempty"`
	Handle []adminOp    `json:"handle,omitempty"`
}

type adminMatch struct {
	Path   []string            `json:"path,omitempty"`
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

// Apply posts 4 routes: main (/slug, /slug/*) plus one Referer-scoped
// asset route per prefix (/css|/js|/assets/* AND Referer ~ /slug).
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

	routes := []adminRoute{{
		ID:    route.ProviderID,
		Match: []adminMatch{{Path: []string{base, base + "/*"}}},
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

	for _, r := range routes {
		if err := c.postRoute(ctx, r); err != nil {
			return fmt.Errorf("proxy: apply route %s: %w", r.ID, err)
		}
	}
	slog.Info("proxy route applied", "provider_id", route.ProviderID, "path", base, "routes", len(routes))
	return nil
}

// Remove deletes the main route plus its asset routes by @id.
// Missing IDs (404) are already-gone, not errors.
// ponytail: best-effort loop; a partial failure returns joined errors and
// the caller decides (Archive keeps the DB row as failed for retry).
func (c *CaddyProvider) Remove(ctx context.Context, providerID string) error {
	if providerID == "" {
		return errors.New("proxy: missing provider route id")
	}
	ids := []string{providerID}
	for _, prefix := range assetPrefixes {
		ids = append(ids, providerID+"-"+prefix)
	}
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
