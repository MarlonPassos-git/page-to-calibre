// probe captures one public article and writes an EPUB for local evaluation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
	"github.com/MarlonPassos-git/page-to-calibre/internal/book"
	"github.com/MarlonPassos-git/page-to-calibre/internal/capture"
)

func main() {
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "uso: probe URL arquivo.epub")
		os.Exit(2)
	}
	if err := run(flag.Arg(0), flag.Arg(1)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(raw, path string) error {
	c := capture.New(capture.Limits{Timeout: 30 * time.Second, MaxBytes: 8 << 20, Redirects: 5})
	ctx := context.Background()
	page, err := c.Fetch(ctx, raw)
	if err != nil {
		return err
	}
	s, err := article.Extract(page.Body, page.URL)
	if err != nil {
		return err
	}
	b, warnings, err := book.Build(ctx, s, c, 30, 32<<20)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"title": s.Metadata.Title, "author": s.Metadata.Author, "source": s.Source, "bytes": len(b), "warnings": warnings})
}
