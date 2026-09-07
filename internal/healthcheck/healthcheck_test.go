package healthcheck

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeClient struct {
	response *http.Response
}

func (f fakeClient) Do(*http.Request) (*http.Response, error) {
	return f.response, nil
}

func TestEngineChecksExpectedStatus(t *testing.T) {
	t.Parallel()

	engine := Engine{Client: fakeClient{response: &http.Response{
		StatusCode: http.StatusNoContent,
		Body:       io.NopCloser(strings.NewReader("")),
	}}}
	result, err := engine.Check(context.Background(), "http://example.test/health", http.StatusNoContent)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Healthy || result.StatusCode != http.StatusNoContent {
		t.Fatalf("result = %+v, want healthy 204", result)
	}
}

func TestEngineDefaultsExpectedStatusToOK(t *testing.T) {
	t.Parallel()

	engine := Engine{Client: fakeClient{response: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
	}}}
	result, err := engine.Check(context.Background(), "http://example.test/health", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Healthy {
		t.Fatalf("result = %+v, want healthy default status", result)
	}
}
