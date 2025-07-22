package click

import (
	"context"
	"crypto/tls"
	"strings"
	"fmt"
	"os"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
)

type Row struct {
	Created time.Time
	Number  uint32
	Title   string
	State   string
	Labels  []string
	Dist    float64
}

type Options struct {
	Host     string
	Port     int
	User     string
	Password string
	DB       string
	Table    string
	TLS      *tls.Config
}

// Search queries ClickHouse with the embedding vector and optional filters.
func Search(vec []float32, state, labels string, opt Options, debug bool) ([]Row, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{opt.Host},
		Auth: clickhouse.Auth{
			Username: opt.User,
			Password: opt.Password,
			Database: opt.DB,
		},
		TLS: opt.TLS,
	})
	if err != nil {
		return nil, err
	}

	// Convert embedding vector to a ClickHouse array literal
	arrBuf := strings.Builder{}
	arrBuf.WriteString("[")
	for i, v := range vec {
		if i > 0 {
			arrBuf.WriteString(",")
		}
		arrBuf.WriteString(fmt.Sprintf("%.7f", v))
	}
	arrBuf.WriteString("]")
	embArrayStr := arrBuf.String()

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf(`
       SELECT created_at, number, title, state, labels,cosineDistance(composite_vec, %s) dist
       FROM %s WHERE 1`, embArrayStr, opt.Table))
	args := []any{}
	argNum := 1
	if state != "" {
		sb.WriteString(fmt.Sprintf(" AND state = $%d", argNum))
		args = append(args, state)
		argNum++
	}
	if labels != "" {
		sb.WriteString(fmt.Sprintf(" AND hasAny(labels, $%d)", argNum))
		args = append(args, strings.Split(labels, ","))
		argNum++
	}
	sb.WriteString(" ORDER BY dist ASC LIMIT 15")

	if debug {
		fmt.Fprintf(os.Stderr, "SQL sent to ClickHouse:\n%s\nArgs: %#v\n", sb.String(), args)
	}

	rows, err := conn.Query(context.Background(), sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var r Row
		if err = rows.Scan(&r.Created, &r.Number, &r.Title, &r.State,
			&r.Labels, &r.Dist); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
