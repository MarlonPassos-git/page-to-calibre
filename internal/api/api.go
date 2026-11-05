// Package api exposes the versioned JSON API independently of the web UI.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
	"github.com/MarlonPassos-git/page-to-calibre/internal/capture"
	"github.com/MarlonPassos-git/page-to-calibre/internal/store"
	"golang.org/x/text/language"
)

type API struct {
	Store      *store.Store
	Origin     string
	PreviewTTL time.Duration
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Stage   string `json:"stage"`
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, code, message, stage string) {
	respond(w, status, map[string]any{"error": apiError{code, message, stage}})
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		fail(w, 400, "invalid_json", "Corpo esperado: objeto JSON válido de até 32 KiB.", "validation")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		fail(w, 400, "invalid_json", "Esperado um único objeto JSON.", "validation")
		return false
	}
	return true
}
func (a *API) Handler(assets http.Handler) http.Handler {
	token := store.ID()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/session", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"csrf_token": token}) })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /api/v1/previews", a.createPreview)
	mux.HandleFunc("GET /api/v1/previews/{id}", a.getPreview)
	mux.HandleFunc("POST /api/v1/imports", a.createImport)
	mux.HandleFunc("GET /api/v1/imports/{id}", a.getImport)
	mux.Handle("/", assets)
	expected, _ := url.Parse(a.Origin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.URL.Path != "/healthz" && (expected == nil || !strings.EqualFold(r.Host, expected.Host)) {
			fail(w, 403, "host_forbidden", "Host não autorizado.", "access")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(token)) != 1 {
				fail(w, 403, "csrf_forbidden", "Token CSRF inválido; recarregue a página.", "access")
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && origin != a.Origin {
				fail(w, 403, "origin_forbidden", "Origem não autorizada.", "access")
				return
			}
			if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
				fail(w, 403, "cross_site_forbidden", "Requisição de outra origem recusada.", "access")
				return
			}
			if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
				fail(w, 415, "json_required", "Content-Type esperado: application/json.", "validation")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *API) createPreview(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &input) {
		return
	}
	u, err := capture.ValidateURL(strings.TrimSpace(input.URL))
	if err != nil {
		fail(w, 400, "invalid_url", err.Error(), "validation")
		return
	}
	p, err := a.Store.CreatePreview(r.Context(), u.String(), a.PreviewTTL)
	if err != nil {
		fail(w, 500, "storage_unavailable", "Não foi possível salvar a prévia.", "storage")
		return
	}
	respond(w, 202, p)
}
func (a *API) getPreview(w http.ResponseWriter, r *http.Request) {
	p, err := a.Store.Preview(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 404, "preview_not_found", "Prévia não encontrada.", "preview")
		return
	}
	if p.ExpiresAt <= time.Now().Unix() {
		fail(w, 410, "preview_expired", store.ErrExpired.Error(), "preview")
		return
	}
	if p.Snapshot != nil {
		copy := *p.Snapshot
		copy.HTML = ""
		p.Snapshot = &copy
	}
	respond(w, 200, p)
}
func (a *API) createImport(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PreviewID string           `json:"preview_id"`
		Metadata  article.Metadata `json:"metadata"`
	}
	if !decode(w, r, &input) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`).MatchString(key) {
		fail(w, 400, "invalid_idempotency_key", "Idempotency-Key esperado: 16 a 128 letras, números, _ ou -.", "validation")
		return
	}
	if err := validateMetadata(input.Metadata); err != nil {
		fail(w, 400, "invalid_metadata", err.Error(), "validation")
		return
	}
	op, err := a.Store.Enqueue(r.Context(), key, input.PreviewID, input.Metadata)
	if err != nil {
		status, code := 409, "preview_not_ready"
		if errors.Is(err, store.ErrExpired) {
			status = 410
			code = "preview_expired"
		}
		if errors.Is(err, store.ErrConflict) {
			code = "idempotency_conflict"
		}
		fail(w, status, code, err.Error(), "enqueue")
		return
	}
	respond(w, 202, op)
}
func (a *API) getImport(w http.ResponseWriter, r *http.Request) {
	op, err := a.Store.Import(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 404, "import_not_found", "Operação não encontrada.", "import")
		return
	}
	respond(w, 200, op)
}
func validateMetadata(m article.Metadata) error {
	if strings.TrimSpace(m.Title) == "" || utf8.RuneCountInString(m.Title) > 500 {
		return errors.New("campo title: esperado texto de 1 a 500 caracteres")
	}
	for _, f := range []struct{ name, value string }{{"author", m.Author}, {"language", m.Language}, {"site", m.Site}, {"description", m.Description}} {
		if utf8.RuneCountInString(f.value) > 4000 {
			return errors.New("campo " + f.name + ": esperado texto de até 4000 caracteres")
		}
	}
	if m.Language != "" {
		if _, err := language.Parse(m.Language); err != nil {
			return fmt.Errorf("campo language=%q: esperado idioma BCP 47, como pt-BR, ou vazio", m.Language)
		}
	}
	if m.Date != "" {
		if _, err := time.Parse("2006-01-02", m.Date); err != nil {
			return fmt.Errorf("campo date=%q: esperado YYYY-MM-DD ou vazio", m.Date)
		}
	}
	if len(m.Tags) > 30 {
		return errors.New("campo tags: esperado até 30 etiquetas")
	}
	for _, tag := range m.Tags {
		if utf8.RuneCountInString(tag) > 100 {
			return errors.New("campo tags: esperado até 100 caracteres por etiqueta")
		}
	}
	return nil
}
