package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/boracomet/go-irmik/irmik/paginate"
)

// Item is the demo resource.
type Item struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type itemStore struct {
	db *sql.DB
}

func newItemStore(db *sql.DB) (*itemStore, error) {
	s := &itemStore{db: db}
	if err := s.migrate(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *itemStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);`)
	if err != nil {
		return fmt.Errorf("migrate items: %w", err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO items (title, body, created_at, updated_at) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
			"Welcome", "First demo item", now, now,
			"HTMX + API", "Edit me from the admin UI or via /api/v1/items", now, now,
		)
		return err
	}
	return nil
}

func (s *itemStore) list(ctx context.Context, p paginate.Params) ([]Item, int, error) {
	where := ""
	args := []any{}
	if p.Q != "" {
		where = "WHERE title LIKE ? OR body LIKE ?"
		q := "%" + p.Q + "%"
		args = append(args, q, q)
	}
	var total int
	countSQL := "SELECT COUNT(*) FROM items " + where
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := p.OrderBy(map[string]string{
		"id":         "id",
		"title":      "title",
		"created_at": "created_at",
		"updated_at": "updated_at",
	})
	if order == "" {
		order = "id DESC"
	}
	q := fmt.Sprintf(`SELECT id, title, body, created_at, updated_at FROM items %s ORDER BY %s LIMIT ? OFFSET ?`, where, order)
	args = append(args, p.Limit(), p.Offset())
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *itemStore) get(ctx context.Context, id int64) (Item, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, body, created_at, updated_at FROM items WHERE id = ?`, id)
	return scanItem(row)
}

func (s *itemStore) create(ctx context.Context, title, body string) (Item, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO items (title, body, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		strings.TrimSpace(title), body, now, now,
	)
	if err != nil {
		return Item{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Item{}, err
	}
	return s.get(ctx, id)
}

func (s *itemStore) update(ctx context.Context, id int64, title, body string) (Item, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE items SET title = ?, body = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(title), body, now, id,
	)
	if err != nil {
		return Item{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Item{}, sql.ErrNoRows
	}
	return s.get(ctx, id)
}

func (s *itemStore) delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(row scanner) (Item, error) {
	var it Item
	var created, updated string
	if err := row.Scan(&it.ID, &it.Title, &it.Body, &created, &updated); err != nil {
		return Item{}, err
	}
	it.CreatedAt, _ = time.Parse(time.RFC3339, created)
	it.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return it, nil
}
