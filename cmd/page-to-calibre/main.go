package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/api"
	"github.com/MarlonPassos-git/page-to-calibre/internal/calibre"
	"github.com/MarlonPassos-git/page-to-calibre/internal/capture"
	"github.com/MarlonPassos-git/page-to-calibre/internal/service"
	"github.com/MarlonPassos-git/page-to-calibre/internal/store"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func number(key string, fallback, min, max int) (int, error) {
	raw := env(key, strconv.Itoa(fallback))
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("%s=%q: esperado inteiro entre %d e %d", key, raw, min, max)
	}
	return n, nil
}
func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		_, port, err := net.SplitHostPort(env("PTC_LISTEN", "127.0.0.1:8091"))
		if err != nil {
			os.Exit(1)
		}
		client := http.Client{Timeout: 3 * time.Second}
		res, err := client.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil {
			os.Exit(1)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application_failed", "error", err.Error())
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	timeout, err := number("PTC_FETCH_TIMEOUT_SECONDS", 30, 1, 120)
	if err != nil {
		return err
	}
	maxBytes, err := number("PTC_MAX_RESOURCE_BYTES", 8<<20, 1024, 32<<20)
	if err != nil {
		return err
	}
	images, err := number("PTC_MAX_IMAGES", 30, 0, 100)
	if err != nil {
		return err
	}
	imageBudget, err := number("PTC_MAX_IMAGE_BYTES", 32<<20, 1024, 256<<20)
	if err != nil {
		return err
	}
	retention, err := number("PTC_RETENTION_DAYS", 30, 1, 365)
	if err != nil {
		return err
	}
	previewMinutes, err := number("PTC_PREVIEW_TTL_MINUTES", 60, 1, 1440)
	if err != nil {
		return err
	}
	origin := env("PTC_ORIGIN", "http://127.0.0.1:8091")
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("PTC_ORIGIN: esperado HTTP(S) com host e sem caminho, credenciais, query ou fragmento")
	}
	library, err := calibre.New(calibre.Config{URL: os.Getenv("CALIBRE_URL"), PublicURL: os.Getenv("CALIBRE_PUBLIC_URL"), Library: os.Getenv("CALIBRE_LIBRARY"), Username: os.Getenv("CALIBRE_USERNAME"), PasswordFile: os.Getenv("CALIBRE_PASSWORD_FILE"), TimeoutSeconds: 60}, nil)
	if err != nil {
		return err
	}
	dir := env("PTC_DATA_DIR", "./data")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(dir, "jobs.sqlite"))
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	worker := service.Worker{Store: db, Fetcher: capture.New(capture.Limits{Timeout: time.Duration(timeout) * time.Second, MaxBytes: int64(maxBytes), Redirects: 5}), Library: library, DataDir: dir, MaxImages: images, ImageBudget: int64(imageBudget), Retention: time.Duration(retention) * 24 * time.Hour, Logger: logger}
	done := make(chan struct{})
	go func() { defer close(done); worker.Run(ctx) }()
	routes := (&api.API{Store: db, Origin: origin, PreviewTTL: time.Duration(previewMinutes) * time.Minute}).Handler(http.FileServer(http.Dir(env("PTC_WEB_DIR", "./web/dist"))))
	server := &http.Server{Addr: env("PTC_LISTEN", "127.0.0.1:8091"), Handler: routes, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	logger.Info("listening", "address", server.Addr)
	err = server.ListenAndServe()
	cancel()
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
