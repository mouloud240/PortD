package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordedRequest struct {
	method string
	path   string
	body   []byte
}

func testServer(t *testing.T, status int, rec **[]recordedRequest) *httptest.Server {
	t.Helper()
	var requests []recordedRequest
	*rec = &requests
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, recordedRequest{method: r.Method, path: r.URL.Path, body: body})
		w.WriteHeader(status)
	}))
}

func TestApplyPostsMainPlusRefererAssetRoutes(t *testing.T) {
	t.Parallel()
	var rec *[]recordedRequest
	server := testServer(t, http.StatusOK, &rec)
	defer server.Close()

	provider := NewCaddyProvider(server.URL)
	err := provider.Apply(context.Background(), Route{ProviderID: "uuid-1", PublicPath: "myapp", UpstreamPort: 7331, Enabled: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(*rec) != 4 {
		t.Fatalf("requests = %d, want 4 (main + css/js/assets)", len(*rec))
	}
	for _, r := range *rec {
		if r.method != http.MethodPost || r.path != "/config/apps/http/servers/srv0/routes" {
			t.Fatalf("request = %s %s, want POST /config/apps/http/servers/srv0/routes", r.method, r.path)
		}
	}

	var main adminRoute
	if err := json.Unmarshal((*rec)[0].body, &main); err != nil {
		t.Fatal(err)
	}
	if main.ID != "uuid-1" || len(main.Match) != 1 || len(main.Match[0].Path) != 2 ||
		main.Match[0].Path[0] != "/myapp" || main.Match[0].Path[1] != "/myapp/*" {
		t.Fatalf("main route = %+v", main)
	}

	for i, prefix := range assetPrefixes {
		var asset adminRoute
		if err := json.Unmarshal((*rec)[i+1].body, &asset); err != nil {
			t.Fatal(err)
		}
		if asset.ID != "uuid-1-"+prefix {
			t.Fatalf("asset id = %q, want %q", asset.ID, "uuid-1-"+prefix)
		}
		match := asset.Match[0]
		if len(match.Path) != 1 || match.Path[0] != "/"+prefix+"/*" {
			t.Fatalf("asset path = %+v", match.Path)
		}
		if got := match.Header["Referer"]; len(got) != 2 || got[0] != "*/myapp/*" || got[1] != "*/myapp" {
			t.Fatalf("asset referer = %+v", match.Header)
		}
		if len(asset.Handle) != 2 || asset.Handle[0].Handler != "rewrite" || asset.Handle[0].StripPathPrefix != "/"+prefix {
			t.Fatalf("asset handle[0] = %+v", asset.Handle)
		}
		if asset.Handle[1].Handler != "reverse_proxy" || asset.Handle[1].Upstreams[0].Dial != "localhost:7331" {
			t.Fatalf("asset handle[1] = %+v", asset.Handle)
		}
	}
}

func TestApplyRejectsInvalidRoutes(t *testing.T) {
	t.Parallel()
	var rec *[]recordedRequest
	server := testServer(t, http.StatusOK, &rec)
	defer server.Close()
	provider := NewCaddyProvider(server.URL)

	for _, route := range []Route{
		{PublicPath: "/x", UpstreamPort: 3000},
		{ProviderID: "id", PublicPath: "", UpstreamPort: 3000},
		{ProviderID: "id", PublicPath: "/x", UpstreamPort: 0},
		{ProviderID: "id", PublicPath: "/x", UpstreamPort: 99999},
	} {
		if err := provider.Apply(context.Background(), route); err == nil {
			t.Fatalf("apply(%+v) = nil, want error", route)
		}
	}
	if len(*rec) != 0 {
		t.Fatalf("requests = %d, want 0", len(*rec))
	}
}

func TestRemoveDeletesRouteFamilyByID(t *testing.T) {
	t.Parallel()
	var rec *[]recordedRequest
	server := testServer(t, http.StatusOK, &rec)
	defer server.Close()

	if err := NewCaddyProvider(server.URL).Remove(context.Background(), "uuid-1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	want := []string{"/id/uuid-1", "/id/uuid-1-css", "/id/uuid-1-js", "/id/uuid-1-assets"}
	if len(*rec) != len(want) {
		t.Fatalf("requests = %d, want %d", len(*rec), len(want))
	}
	for i, path := range want {
		if (*rec)[i].method != http.MethodDelete || (*rec)[i].path != path {
			t.Fatalf("request %d = %s %s, want DELETE %s", i, (*rec)[i].method, (*rec)[i].path, path)
		}
	}
}

func TestRemoveIgnoresMissingRoutes(t *testing.T) {
	t.Parallel()
	var rec *[]recordedRequest
	server := testServer(t, http.StatusNotFound, &rec)
	defer server.Close()

	if err := NewCaddyProvider(server.URL).Remove(context.Background(), "gone"); err != nil {
		t.Fatalf("remove missing = %v, want nil", err)
	}
}

func TestRemoveRejectsEmptyID(t *testing.T) {
	t.Parallel()
	if err := NewCaddyProvider("http://localhost:2019").Remove(context.Background(), ""); err == nil {
		t.Fatal("remove(\"\") = nil, want error")
	}
}

func TestPublicPathNormalized(t *testing.T) {
	t.Parallel()
	var rec *[]recordedRequest
	server := testServer(t, http.StatusOK, &rec)
	defer server.Close()

	if err := NewCaddyProvider(server.URL).Apply(context.Background(), Route{ProviderID: "u", PublicPath: "/myapp/", UpstreamPort: 3000}); err != nil {
		t.Fatal(err)
	}
	body := string((*rec)[0].body)
	if !strings.Contains(body, `"/myapp"`) || !strings.Contains(body, `"/myapp/*"`) {
		t.Fatalf("main match not normalized: %s", body)
	}
}
