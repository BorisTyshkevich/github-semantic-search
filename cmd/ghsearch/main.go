package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/yourname/ghsearch/internal/click"
	"github.com/yourname/ghsearch/internal/embed"
)

func main() {
	var (
        version   = "dev"
        buildDate = "unknown"
		chHost = flag.String("host", "localhost:9000", "ClickHouse host:port")
		chUser = flag.String("user", "default", "ClickHouse user")
		chPass = flag.String("pass", "", "ClickHouse password")
		chDB   = flag.String("db",   "github", "ClickHouse database")
		chTab  = flag.String("table","clickcomments","ClickHouse table")
		state  = flag.String("state",  "", "filter by issue state")
		labels = flag.String("labels", "", "comma-separated label filter")
		debug  = flag.Bool("debug",   false, "print embedding")
	)
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("query text required")
	}
	query := strings.Join(flag.Args(), " ")

	vec, err := embed.Vector(query)
	if err != nil {
		log.Fatalf("embedding: %v", err)
	}
	if *debug {
		fmt.Printf("dims=%d first=%.4f\n", len(vec), vec[0])
	}

	rows, err := click.Search(vec, *state, *labels, click.Options{
		Host: *chHost, User: *chUser, Password: *chPass,
		DB: *chDB, Table: *chTab,
	})
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	seen := map[int]struct{}{}
	for _, r := range rows {
		if _, ok := seen[r.Number]; ok {
			continue
		}
		seen[r.Number] = struct{}{}
		url := fmt.Sprintf("https://github.com/ClickHouse/ClickHouse/issues/%d", r.Number)
		fmt.Printf("%s %.4f %6s  \x1b]8;;%s\a#%d\x1b]8;;\a  (%s) [%s]\n",
			r.Created, r.Dist, r.State, url, r.Number,
			r.Title, strings.Join(r.Labels, ","))
	}
}