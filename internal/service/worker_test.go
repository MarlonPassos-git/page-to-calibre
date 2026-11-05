package service

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
	"github.com/MarlonPassos-git/page-to-calibre/internal/calibre"
	"github.com/MarlonPassos-git/page-to-calibre/internal/capture"
	"github.com/MarlonPassos-git/page-to-calibre/internal/store"
)

type fixtureFetcher struct{ calls int }

func (f *fixtureFetcher) Fetch(context.Context, string) (capture.Resource, error) {
	f.calls++
	u, _ := url.Parse("https://example.org/story")
	return capture.Resource{URL: u, ContentType: "text/html", Body: []byte(`<html><head><title>Artigo</title></head><body><article><h1>Artigo</h1>` + strings.Repeat(`<p>Este artigo fala sobre leitura e preservação de conteúdo para consulta posterior. Acentos e parágrafos devem ser mantidos no EPUB.</p>`, 8) + `</article></body></html>`)}, nil
}

type fixtureLibrary struct {
	found *calibre.Book
	adds  int
}

func (l *fixtureLibrary) Find(context.Context, string) (*calibre.Book, error) { return l.found, nil }
func (l *fixtureLibrary) Ensure(context.Context, string, string) (calibre.Book, error) {
	l.adds++
	l.found = &calibre.Book{ID: 12, Title: "Título editado", Formats: []string{"EPUB"}}
	return *l.found, nil
}
func TestPersistentJobReusesSnapshotAndRecoversResult(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.sqlite")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixtureFetcher{}
	library := &fixtureLibrary{}
	w := Worker{Store: db, Fetcher: f, Library: library, DataDir: dir, MaxImages: 5, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	p, err := db.CreatePreview(ctx, "https://example.org/story", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	w.Prepare(ctx, p.ID)
	p, _ = db.Preview(ctx, p.ID)
	m := p.Snapshot.Metadata
	m.Title = "Título editado"
	op, err := db.Enqueue(ctx, "idempotency-key-123456", p.ID, m)
	if err != nil {
		t.Fatal(err)
	}
	// A disconnected browser leaves a durable queued operation; reopen the DB to
	// simulate an entirely new process rather than invoking through the request.
	db.Close()
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w.Store = db
	w.Import(ctx, op.ID)
	op, err = db.Import(ctx, op.ID)
	if err != nil || op.State != "succeeded" || op.Result.ID != 12 || f.calls != 1 || library.adds != 1 {
		t.Fatal(op, err, f.calls, library.adds)
	}
	same, err := db.Enqueue(ctx, "idempotency-key-123456", p.ID, m)
	if err != nil || same.ID != op.ID {
		t.Fatal(same, err)
	}
	if err = db.Cleanup(ctx, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Import(ctx, op.ID); err != nil {
		t.Fatal("recent result removed", err)
	}
}
func TestInterruptedWriteReconcilesWithoutAdding(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncertain", true: "confirmed"}[confirmed], func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			db, _ := store.Open(filepath.Join(dir, "jobs.sqlite"))
			defer db.Close()
			p, _ := db.CreatePreview(ctx, "https://example.org/story", time.Hour)
			p.State = "ready"
			p.Snapshot = &article.Snapshot{Identity: strings.Repeat("a", 64)}
			db.SavePreview(ctx, p)
			op, err := db.Enqueue(ctx, "recovery-key-123456", p.ID, article.Metadata{Title: "Recovered"})
			if err != nil {
				t.Fatal(err)
			}
			op.State = "running"
			op.Stage = "writing"
			db.SaveImport(ctx, op)
			library := &fixtureLibrary{}
			if confirmed {
				library.found = &calibre.Book{ID: 8, Formats: []string{"EPUB"}}
			}
			artifact := filepath.Join(dir, "import-"+op.ID+".epub")
			if err := os.WriteFile(artifact, []byte("exact sent EPUB"), 0600); err != nil {
				t.Fatal(err)
			}
			w := Worker{Store: db, DataDir: dir, Library: library, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
			w.Import(ctx, op.ID)
			op, _ = db.Import(ctx, op.ID)
			_, artifactErr := os.Stat(artifact)
			if confirmed && !os.IsNotExist(artifactErr) {
				t.Fatal("confirmed artifact retained")
			}
			if !confirmed && artifactErr != nil {
				t.Fatal("uncertain artifact lost")
			}
			expected := "uncertain"
			if confirmed {
				expected = "succeeded"
			}
			if op.State != expected || library.adds != 0 {
				t.Fatal(op, library.adds)
			}
		})
	}
}

func TestNewKeyCannotBypassUncertainOrigin(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, _ := store.Open(filepath.Join(dir, "jobs.sqlite"))
	defer db.Close()
	p, _ := db.CreatePreview(ctx, "https://example.org/story", time.Hour)
	p.State = "ready"
	p.Snapshot = &article.Snapshot{Identity: strings.Repeat("a", 64)}
	db.SavePreview(ctx, p)
	first, _ := db.Enqueue(ctx, "first-key-123456789", p.ID, article.Metadata{Title: "Original"})
	first.State = "uncertain"
	first.Stage = "writing"
	db.SaveImport(ctx, first)
	second, _ := db.Enqueue(ctx, "second-key-12345678", p.ID, article.Metadata{Title: "Second"})
	library := &fixtureLibrary{}
	w := Worker{Store: db, Library: library, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	w.Import(ctx, second.ID)
	second, _ = db.Import(ctx, second.ID)
	if second.State != "uncertain" || second.Stage != "waiting_for_origin" || library.adds != 0 {
		t.Fatal(second, library.adds)
	}
	library.found = &calibre.Book{ID: 1, Formats: []string{"EPUB"}}
	w.Import(ctx, first.ID)
	w.Import(ctx, second.ID)
	second, _ = db.Import(ctx, second.ID)
	if second.State != "succeeded" || !second.Result.Duplicate || library.adds != 0 {
		t.Fatal(second, library.adds)
	}
}

func TestArtifactCleanupPreservesActiveAndUncertainJobs(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, _ := store.Open(filepath.Join(dir, "jobs.sqlite"))
	defer db.Close()
	p, _ := db.CreatePreview(ctx, "https://example.org/story", time.Hour)
	p.State = "ready"
	p.Snapshot = &article.Snapshot{}
	db.SavePreview(ctx, p)
	op, _ := db.Enqueue(ctx, "cleanup-key-123456", p.ID, article.Metadata{Title: "Active"})
	active := filepath.Join(dir, "import-"+op.ID+".epub")
	orphan := filepath.Join(dir, "import-"+store.ID()+".epub")
	other := filepath.Join(dir, "keep.txt")
	for _, path := range []string{active, orphan, other} {
		os.WriteFile(path, []byte("fixture"), 0600)
	}
	w := Worker{Store: db, DataDir: dir, Retention: time.Hour}
	if err := w.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("expired artifact retained")
	}
	for _, path := range []string{active, other} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("active/unrelated file deleted", err)
		}
	}
	op.State = "uncertain"
	db.SaveImport(ctx, op)
	w.Cleanup(ctx)
	if _, err := os.Stat(active); err != nil {
		t.Fatal("uncertain artifact deleted", err)
	}
}
