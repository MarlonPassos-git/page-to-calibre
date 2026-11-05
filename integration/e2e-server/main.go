// This executable is a local test harness, excluded from the runtime image.
// It replaces ONLY public-page capture; SQLite, API, worker, EPUB and Calibre
// stay real. It never loads personal credentials or library paths.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/api"
	"github.com/MarlonPassos-git/page-to-calibre/internal/calibre"
	"github.com/MarlonPassos-git/page-to-calibre/internal/capture"
	"github.com/MarlonPassos-git/page-to-calibre/internal/service"
	"github.com/MarlonPassos-git/page-to-calibre/internal/store"
)

type fixture struct{}

func (fixture) Fetch(_ context.Context, raw string) (capture.Resource, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return capture.Resource{}, err
	}
	return capture.Resource{URL: u, ContentType: "text/html", Body: []byte(`<html lang="pt-BR"><head><title>Leitura de teste</title><meta name="author" content="Autora de teste"></head><body><article><h1>Leitura de teste</h1>` + strings.Repeat(`<p>Este artigo sintético existe somente para testar o envio de uma leitura ao Calibre. Seu conteúdo não contém dados pessoais nem credenciais.</p>`, 10) + `<pre><code>fmt.Println(&quot;Olá&quot;)</code></pre></article></body></html>`)}, nil
}
func main() {
	container := os.Getenv("PTC_CONTRACT_CONTAINER")
	if container == "" {
		panic("PTC_CONTRACT_CONTAINER required")
	}
	dir, err := os.MkdirTemp("", "ptc-e2e-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	db, err := store.Open(filepath.Join(dir, "jobs.sqlite"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	runner := func(ctx context.Context, args ...string) ([]byte, error) {
		if args[0] == "add" {
			path := args[len(args)-1]
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			copy := exec.CommandContext(ctx, "docker", "exec", "-i", container, "sh", "-c", "cat > /tmp/e2e.epub")
			copy.Stdin = file
			if out, err := copy.CombinedOutput(); err != nil {
				return out, err
			}
			args[len(args)-1] = "/tmp/e2e.epub"
		}
		return exec.CommandContext(ctx, "docker", append([]string{"exec", container, "calibredb"}, args...)...).Output()
	}
	library, err := calibre.New(calibre.Config{URL: "http://127.0.0.1:8080", Library: "library", Username: "writer", PasswordFile: "/tmp/password"}, runner)
	if err != nil {
		panic(err)
	}
	w := service.Worker{Store: db, Fetcher: fixture{}, Library: library, DataDir: dir, MaxImages: 5, Retention: 24 * time.Hour, Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	handler := (&api.API{Store: db, Origin: "http://127.0.0.1:8092", PreviewTTL: time.Hour}).Handler(http.FileServer(http.Dir("web/dist")))
	server := http.Server{Addr: "127.0.0.1:8092", Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if err = server.ListenAndServe(); err != nil {
		panic(err)
	}
}
