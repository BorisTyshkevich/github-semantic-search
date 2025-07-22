package server

import (
	"encoding/json"
	"net/http"

	"github.com/BorisTyshkevich/github-semantic-search/internal/click"
	"github.com/BorisTyshkevich/github-semantic-search/internal/embed"
	"github.com/BorisTyshkevich/github-semantic-search/web"
)

var content = web.FS

var SearchFn = click.Search

func Handler(emb embed.Embedder, opt click.Options, debug bool) http.Handler {
	mux := http.NewServeMux()
	fs := http.FS(content)
	mux.Handle("/", http.FileServer(fs))
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		if q == "" {
			http.Error(w, "missing query", http.StatusBadRequest)
			return
		}
		state := r.URL.Query().Get("state")
		labels := r.URL.Query().Get("labels")

		vec, err := emb.Vector(q, debug)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rows, err := SearchFn(vec, state, labels, opt, debug)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rows)
	})
	return mux
}
