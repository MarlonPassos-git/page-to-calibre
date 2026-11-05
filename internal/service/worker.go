// Package service coordinates durable preparation and serialized imports.
package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
	"github.com/MarlonPassos-git/page-to-calibre/internal/book"
	"github.com/MarlonPassos-git/page-to-calibre/internal/calibre"
	"github.com/MarlonPassos-git/page-to-calibre/internal/store"
)

type Library interface {
	Find(context.Context, string) (*calibre.Book, error)
	Ensure(context.Context, string, string) (calibre.Book, error)
}
type Worker struct {
	Store       *store.Store
	Fetcher     book.Fetcher
	Library     Library
	DataDir     string
	MaxImages   int
	Retention   time.Duration
	Logger      *slog.Logger
	ImageBudget int64
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	clean := time.NewTicker(time.Hour)
	defer clean.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-clean.C:
			if err := w.Cleanup(ctx); err != nil {
				w.Logger.Error("cleanup_failed")
			}
		case <-ticker.C:
			if id, err := w.Store.Next(ctx, "import"); err == nil {
				w.Import(ctx, id)
				continue
			}
			if id, err := w.Store.Next(ctx, "preview"); err == nil {
				w.Prepare(ctx, id)
			}
		}
	}
}
func (w *Worker) Prepare(ctx context.Context, id string) {
	p, err := w.Store.Preview(ctx, id)
	if err != nil {
		return
	}
	page, err := w.Fetcher.Fetch(ctx, p.URL)
	if err == nil && !strings.Contains(page.ContentType, "html") {
		err = errors.New("origem não retornou HTML")
	}
	var snapshot article.Snapshot
	if err == nil {
		snapshot, err = article.Extract(page.Body, page.URL)
	}
	if err != nil {
		p.State = "failed"
		p.Error = "Não foi possível preparar um artigo legível. A origem pode estar bloqueada, indisponível ou exigir JavaScript."
	} else {
		p.State = "ready"
		snapshot.OriginalURL = p.URL
		p.Snapshot = &snapshot
	}
	if e := w.Store.SavePreview(ctx, p); e != nil {
		w.Logger.Error("preview_save_failed", "id", id)
	}
}
func (w *Worker) Import(ctx context.Context, id string) {
	op, err := w.Store.Import(ctx, id)
	if err != nil {
		return
	}
	p, err := w.Store.Preview(ctx, op.PreviewID)
	if err != nil || p.Snapshot == nil {
		w.finish(ctx, op, "failed", "Snapshot da prévia indisponível.")
		return
	}
	snapshot := *p.Snapshot
	snapshot.Metadata = op.Metadata
	if op.Stage != "writing" {
		blocked, err := w.Store.OriginUncertain(ctx, snapshot.Identity, op.ID)
		if err != nil {
			return
		}
		if blocked {
			op.Stage = "waiting_for_origin"
			w.finish(ctx, op, "uncertain", "Outra operação desta origem aguarda confirmação no Calibre.")
			return
		}
	}
	if op.Stage == "writing" {
		existing, err := w.Library.Find(ctx, snapshot.Identity)
		if err != nil || existing == nil {
			w.finish(ctx, op, "uncertain", calibre.ErrUncertain.Error())
			return
		}
		op.Result = existing
		if w.finish(ctx, op, "succeeded", "") {
			_ = os.Remove(filepath.Join(w.DataDir, "import-"+op.ID+".epub"))
		}
		return
	}
	existing, err := w.Library.Find(ctx, snapshot.Identity)
	if err != nil {
		w.finish(ctx, op, "failed", err.Error())
		return
	}
	if existing != nil {
		existing.Duplicate = true
		op.Result = existing
		w.finish(ctx, op, "succeeded", "")
		return
	}
	op.State = "running"
	op.Stage = "packaging"
	if err = w.Store.SaveImport(ctx, op); err != nil {
		return
	}
	epub, warnings, err := book.Build(ctx, snapshot, w.Fetcher, w.MaxImages, w.ImageBudget)
	if err != nil {
		w.finish(ctx, op, "failed", "Falha ao gerar EPUB.")
		return
	}
	op.Warnings = warnings
	path := filepath.Join(w.DataDir, "import-"+op.ID+".epub")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		w.finish(ctx, op, "failed", "Falha ao preparar arquivo temporário.")
		return
	}
	_, err = file.Write(epub)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(path)
		w.finish(ctx, op, "failed", "Falha ao gravar EPUB.")
		return
	}
	op.Stage = "writing"
	if err = w.Store.SaveImport(ctx, op); err != nil {
		return
	}
	result, err := w.Library.Ensure(ctx, snapshot.Identity, filepath.Clean(path))
	if err != nil {
		state := "failed"
		if errors.Is(err, calibre.ErrUncertain) {
			state = "uncertain"
		}
		w.finish(ctx, op, state, err.Error())
		return
	}
	op.Result = &result
	if w.finish(ctx, op, "succeeded", "") {
		_ = os.Remove(path)
	}
}
func (w *Worker) finish(ctx context.Context, op store.Import, state, message string) bool {
	op.State = state
	op.Error = message
	if state == "succeeded" {
		op.Stage = "confirmed"
	}
	if err := w.Store.SaveImport(ctx, op); err != nil {
		w.Logger.Error("import_save_failed", "id", op.ID)
		return false
	} else {
		w.Logger.Info("import_state", "id", op.ID, "state", state, "stage", op.Stage)
	}
	return true
}

// Cleanup removes only generated artifacts whose retained operation was already
// deleted. Active and uncertain jobs keep their exact EPUB for reconciliation.
func (w *Worker) Cleanup(ctx context.Context) error {
	if err := w.Store.Cleanup(ctx, w.Retention); err != nil {
		return err
	}
	entries, err := os.ReadDir(w.DataDir)
	if err != nil {
		return err
	}
	pattern := regexp.MustCompile(`^import-([0-9a-f]{48})\.epub$`)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := pattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		if _, err = w.Store.Import(ctx, match[1]); errors.Is(err, sql.ErrNoRows) {
			if err = os.Remove(filepath.Join(w.DataDir, entry.Name())); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}
