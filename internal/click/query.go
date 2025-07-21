package click

import (
	"context"
	"database/sql"
	"strings"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
)

type Row struct {
	Created string
	Number  int
	Title   string
	Actor   string
	State   string
	Labels  []string
	Dist    float32
}

type Options struct {
	Host     string
	Port     int
	User     string
	Password string
	DB       string
	Table    string
}

// Search queries ClickHouse with the embedding vector and optional filters.
func Search(vec []float32, state, labels string, opt Options) ([]Row, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{opt.Host},
		Auth: clickhouse.Auth{
			Username: opt.User,
			Password: opt.Password,
			Database: opt.DB,
		},
	})
	if err != nil {
		return nil, err
	}

	sb := strings.Builder{}
	sb.WriteString(`
SELECT created_at, number, title, actor_login, state, labels,
       cosineDistance(composite_vec, $1) dist
FROM ` + opt.Table + ` WHERE 1`)
	args := []any{vec}

	if state != "" {
		sb.WriteString(" AND state = $2")
		args = append(args, state)
	}
	if labels != "" {
		sb.WriteString(" AND hasAny(labels, $3)")
		args = append(args, strings.Split(labels, ","))
	}
	sb.WriteString(" ORDER BY dist ASC LIMIT 20")

	rows, err := conn.Query(context.Background(), sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var r Row
		if err = rows.Scan(&r.Created, &r.Number, &r.Title, &r.Actor, &r.State,
			&r.Labels, &r.Dist); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}