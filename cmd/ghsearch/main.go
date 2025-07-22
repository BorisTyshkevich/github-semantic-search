package main

import (
	"crypto/tls"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BorisTyshkevich/github-semantic-search/internal/click"
	"github.com/BorisTyshkevich/github-semantic-search/internal/embed"
)

func main() {
	var (
		version     = "dev"
		buildDate   = "unknown"
		showVersion = flag.Bool("version", false, "print version and build date")
		connName    = flag.String("connection", "", "name of connection from ~/.clickhouse-client/config.xml (optional)")
		host        = flag.String("host", "github.demo.altinity.cloud", "ClickHouse host")
		port        = flag.Int("port", 9440, "ClickHouse port")
		user        = flag.String("user", "demo", "ClickHouse user")
		pass        = flag.String("pass", "demo", "ClickHouse password")
		secure      = flag.Bool("secure", true, "use TLS")
		chTab       = flag.String("table", "clickcomments", "ClickHouse table")
		state       = flag.String("state", "", "filter by issue state")
		labels      = flag.String("labels", "", "comma-separated label filter")
		debug       = flag.Bool("debug", false, "print embedding")
	)
	flag.Parse()
	if *showVersion {
		fmt.Printf("ghsearch version %s (%s)\n", version, buildDate)
		return
	}
	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] query\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}
	query := strings.Join(flag.Args(), " ")

	vec, err := embed.Vector(query, *debug)
	if err != nil {
		log.Fatalf("embedding: %v", err)
	}
	if *debug {
		fmt.Printf("dims=%d first=%.4f\n", len(vec), vec[0])
	}

	var (
		clickHost  string
		clickUser  string
		clickPass  string
		clickDB    string
		useSecure  bool
	)
	if *connName != "" {
		var err error
		clickHost, clickUser, clickPass, clickDB, useSecure, err = loadConnection(*connName)
		if err != nil {
			log.Fatalf("load connection: %v", err)
		}
	} else {
		clickHost = fmt.Sprintf("%s:%d", *host, *port)
		clickUser = *user
		clickPass = *pass
		clickDB = "default"
		useSecure = *secure
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: true}
	if !useSecure {
		tlsCfg = nil
	}

	rows, err := click.Search(vec, *state, *labels, click.Options{
		Host: clickHost, User: clickUser, Password: clickPass, DB: clickDB, Table: *chTab, TLS: tlsCfg,
	}, *debug)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	seen := map[uint32]struct{}{}
	for _, r := range rows {
		if _, ok := seen[r.Number]; ok {
			continue
		}
		seen[r.Number] = struct{}{}
		url := fmt.Sprintf("https://github.com/ClickHouse/ClickHouse/issues/%d", r.Number)
		dateStr := r.Created.Format("2006-01-02")
		fmt.Printf("%s \x1b]8;;%s\a#%d\x1b]8;;\a %.4f %6s  (%s) [%s]\n",
			dateStr, url, r.Number, r.Dist, r.State,
			r.Title, strings.Join(r.Labels, ","))
	}
}


type clickhouseConfig struct {
	Connections struct {
		Connections []struct {
			Name     string `xml:"name"`
			Hostname string `xml:"hostname"`
			Port     int    `xml:"port"`
			User     string `xml:"user"`
			Password string `xml:"password"`
			Database string `xml:"database"`
			Secure   int    `xml:"secure"`
		} `xml:"connection"`
	} `xml:"connections_credentials"`
}

func loadConnection(name string) (host, user, pass, db string, secure bool, err error) {
	path := filepath.Join(os.Getenv("HOME"), ".clickhouse-client", "config.xml")
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return "", "", "", "", false, err
	}
	var cfg clickhouseConfig
	if err := xml.Unmarshal(raw, &cfg); err != nil {
		return "", "", "", "", false, err
	}
	for _, c := range cfg.Connections.Connections {
		if c.Name == name {
			return fmt.Sprintf("%s:%d", c.Hostname, c.Port), c.User, c.Password, c.Database, c.Secure == 1, nil
		}
	}
	return "", "", "", "", false, fmt.Errorf("connection %q not found", name)
}
