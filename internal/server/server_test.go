package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BorisTyshkevich/github-semantic-search/internal/click"
	"github.com/BorisTyshkevich/github-semantic-search/internal/embed"
)

func TestHandler(t *testing.T) {
	SearchFn = func(vec []float32, state, labels string, opt click.Options, debug bool) ([]click.Row, error) {
		return []click.Row{{Number: 1, Title: "Test", State: "open"}}, nil
	}
	defer func() { SearchFn = click.Search }()
	h := Handler(embed.Mock{}, click.Options{}, false)
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/search?query=foo")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var rows []click.Row
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Number != 1 {
		t.Fatalf("unexpected response: %#v", rows)
	}
}
