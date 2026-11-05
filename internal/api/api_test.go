package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/store"
)

func TestSameOriginJSONAndCSRF(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "jobs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := (&API{Store: db, Origin: "http://app.test", PreviewTTL: time.Hour}).Handler(http.NotFoundHandler())
	session := httptest.NewRecorder()
	h.ServeHTTP(session, httptest.NewRequest("GET", "http://app.test/api/v1/session", nil))
	var body map[string]string
	json.Unmarshal(session.Body.Bytes(), &body)
	token := body["csrf_token"]
	if token == "" {
		t.Fatal("missing CSRF token")
	}
	for _, tc := range []struct {
		name, host, origin, contentType, token string
		want                                   int
	}{{"valid", "app.test", "http://app.test", "application/json", token, 202}, {"wrong origin", "app.test", "https://attacker.test", "application/json", token, 403}, {"wrong host", "attacker.test", "", "application/json", token, 403}, {"missing token", "app.test", "http://app.test", "application/json", "", 403}, {"simple form", "app.test", "http://app.test", "text/plain", token, 415}} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+tc.host+"/api/v1/previews", strings.NewReader(`{"url":"https://example.org/story"}`))
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Content-Type", tc.contentType)
			r.Header.Set("X-CSRF-Token", tc.token)
			res := httptest.NewRecorder()
			h.ServeHTTP(res, r)
			if res.Code != tc.want {
				t.Fatal(res.Code, res.Body.String())
			}
		})
	}
}
