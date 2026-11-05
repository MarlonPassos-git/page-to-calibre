package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
)

func TestRetentionPreservesActiveAndUncertainSnapshots(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, _ := s.CreatePreview(ctx, "https://example.org", time.Hour)
	p.State = "ready"
	p.Snapshot = &article.Snapshot{}
	s.SavePreview(ctx, p)
	op, err := s.Enqueue(ctx, "key1234567890123456", p.ID, article.Metadata{Title: "Article"})
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec("UPDATE previews SET expires=0")
	if err = s.Cleanup(ctx, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Preview(ctx, p.ID); err != nil {
		t.Fatal("active snapshot deleted")
	}
	op.State = "uncertain"
	s.SaveImport(ctx, op)
	s.Cleanup(ctx, time.Hour)
	if _, err = s.Preview(ctx, p.ID); err != nil {
		t.Fatal("uncertain snapshot deleted")
	}
}
func TestIdempotencyRejectsChangedPayload(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(filepath.Join(t.TempDir(), "db.sqlite"))
	defer s.Close()
	p, _ := s.CreatePreview(ctx, "https://example.org", time.Hour)
	p.State = "ready"
	p.Snapshot = &article.Snapshot{}
	s.SavePreview(ctx, p)
	s.Enqueue(ctx, "key1234567890123456", p.ID, article.Metadata{Title: "One"})
	_, err := s.Enqueue(ctx, "key1234567890123456", p.ID, article.Metadata{Title: "Two"})
	if !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
