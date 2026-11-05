// Package store persists snapshots and import jobs in one local SQLite database.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
	"github.com/MarlonPassos-git/page-to-calibre/internal/calibre"
	_ "modernc.org/sqlite"
)

var ErrExpired = errors.New("prévia expirada; prepare novamente preservando as edições")
var ErrConflict = errors.New("chave de idempotência já utilizada para outro conteúdo")

type Preview struct {
	ID        string            `json:"id"`
	URL       string            `json:"url"`
	State     string            `json:"state"`
	Snapshot  *article.Snapshot `json:"snapshot,omitempty"`
	Error     string            `json:"error,omitempty"`
	ExpiresAt int64             `json:"expires_at"`
}
type Import struct {
	ID        string           `json:"id"`
	PreviewID string           `json:"preview_id"`
	Key       string           `json:"-"`
	State     string           `json:"state"`
	Stage     string           `json:"stage"`
	Metadata  article.Metadata `json:"metadata"`
	Result    *calibre.Book    `json:"result,omitempty"`
	Error     string           `json:"error,omitempty"`
	Warnings  []string         `json:"warnings"`
	UpdatedAt int64            `json:"updated_at"`
}
type Store struct{ db *sql.DB }

func ID() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS previews(id TEXT PRIMARY KEY,state TEXT NOT NULL,expires INTEGER NOT NULL,body BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS imports(id TEXT PRIMARY KEY,key TEXT NOT NULL UNIQUE,preview_id TEXT NOT NULL,state TEXT NOT NULL,updated INTEGER NOT NULL,body BLOB NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) CreatePreview(ctx context.Context, raw string, ttl time.Duration) (Preview, error) {
	p := Preview{ID: ID(), URL: raw, State: "queued", ExpiresAt: time.Now().Add(ttl).Unix()}
	b, _ := json.Marshal(p)
	_, err := s.db.ExecContext(ctx, "INSERT INTO previews VALUES(?,?,?,?)", p.ID, p.State, p.ExpiresAt, b)
	return p, err
}
func (s *Store) SavePreview(ctx context.Context, p Preview) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE previews SET state=?,body=? WHERE id=?", p.State, b, p.ID)
	return err
}
func (s *Store) Preview(ctx context.Context, id string) (Preview, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM previews WHERE id=?", id).Scan(&b)
	var p Preview
	if err == nil {
		err = json.Unmarshal(b, &p)
	}
	return p, err
}
func (s *Store) Import(ctx context.Context, id string) (Import, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM imports WHERE id=?", id).Scan(&b)
	var op Import
	if err == nil {
		err = json.Unmarshal(b, &op)
	}
	return op, err
}
func (s *Store) Enqueue(ctx context.Context, key, previewID string, metadata article.Metadata) (Import, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Import{}, err
	}
	defer tx.Rollback()
	var old []byte
	err = tx.QueryRowContext(ctx, "SELECT body FROM imports WHERE key=?", key).Scan(&old)
	if err == nil {
		var op Import
		if err = json.Unmarshal(old, &op); err != nil {
			return Import{}, err
		}
		a, _ := json.Marshal(op.Metadata)
		b, _ := json.Marshal(metadata)
		if op.PreviewID != previewID || string(a) != string(b) {
			return Import{}, ErrConflict
		}
		return op, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Import{}, err
	}
	var b []byte
	if err = tx.QueryRowContext(ctx, "SELECT body FROM previews WHERE id=?", previewID).Scan(&b); err != nil {
		return Import{}, err
	}
	var p Preview
	if err = json.Unmarshal(b, &p); err != nil {
		return Import{}, err
	}
	if p.ExpiresAt <= time.Now().Unix() {
		return Import{}, ErrExpired
	}
	if p.State != "ready" || p.Snapshot == nil {
		return Import{}, errors.New("prévia ainda não está pronta")
	}
	op := Import{ID: ID(), PreviewID: previewID, Key: key, State: "queued", Stage: "queued", Metadata: metadata, Warnings: []string{}, UpdatedAt: time.Now().Unix()}
	b, _ = json.Marshal(op)
	if _, err = tx.ExecContext(ctx, "INSERT INTO imports VALUES(?,?,?,?,?,?)", op.ID, key, previewID, op.State, op.UpdatedAt, b); err != nil {
		return Import{}, err
	}
	return op, tx.Commit()
}
func (s *Store) SaveImport(ctx context.Context, op Import) error {
	op.UpdatedAt = time.Now().Unix()
	b, err := json.Marshal(op)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE imports SET state=?,updated=?,body=? WHERE id=?", op.State, op.UpdatedAt, b, op.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("operação não encontrada")
	}
	return nil
}
func (s *Store) Next(ctx context.Context, kind string) (string, error) {
	query := "SELECT id FROM previews WHERE state='queued' ORDER BY rowid LIMIT 1"
	var args []any
	if kind == "import" {
		query = "SELECT id FROM imports WHERE state IN ('queued','running') OR (state='uncertain' AND updated<?) ORDER BY CASE WHEN state='uncertain' THEN 1 ELSE 0 END,rowid LIMIT 1"
		args = []any{time.Now().Add(-time.Minute).Unix()}
	}
	var id string
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&id)
	return id, err
}

// Cleanup never removes snapshots referenced by active or uncertain jobs.
func (s *Store) Cleanup(ctx context.Context, retention time.Duration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM imports WHERE state IN ('succeeded','failed') AND updated < ?", time.Now().Add(-retention).Unix()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM previews WHERE expires < ? AND id NOT IN (SELECT preview_id FROM imports)", time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

// OriginUncertain prevents a new key from bypassing an earlier ambiguous write.
func (s *Store) OriginUncertain(ctx context.Context, identity, exclude string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM imports i JOIN previews p ON i.preview_id=p.id
 WHERE i.id<>? AND i.state IN ('running','uncertain') AND json_extract(i.body,'$.stage')='writing'
 AND json_extract(p.body,'$.snapshot.identity')=?)`, exclude, identity).Scan(&exists)
	return exists, err
}
