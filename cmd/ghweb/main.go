package main

import (
	"crypto/tls"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BorisTyshkevich/github-semantic-search/internal/click"
	"github.com/BorisTyshkevich/github-semantic-search/internal/embed"
	"github.com/BorisTyshkevich/github-semantic-search/internal/server"
)

var searchFn = click.Search

func main() {
	var (
		listen   = flag.String("listen", ":8080", "http listen address")
		connName = flag.String("connection", "", "name of connection from ~/.clickhouse-client/config.xml")
		host     = flag.String("host", "github.demo.altinity.cloud", "ClickHouse host")
		port     = flag.Int("port", 9440, "ClickHouse port")
		user     = flag.String("user", "demo", "ClickHouse user")
		pass     = flag.String("pass", "demo", "ClickHouse password")
		secure   = flag.Bool("secure", true, "use TLS")
		chTab    = flag.String("table", "clickcomments", "ClickHouse table")
		debug    = flag.Bool("debug", false, "debug mode")
	)
	flag.Parse()

	var (
		clickHost string
		clickUser string
		clickPass string
		clickDB   string
		useSecure bool
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

	emb := embed.OpenAI{}
	handler := server.Handler(emb, click.Options{
		Host:     clickHost,
		User:     clickUser,
		Password: clickPass,
		DB:       clickDB,
		Table:    *chTab,
		TLS:      tlsCfg,
	}, *debug)

	log.Printf("listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, handler))
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
	raw, err := os.ReadFile(path)
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
