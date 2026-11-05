package book

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
	"github.com/MarlonPassos-git/page-to-calibre/internal/capture"
)

type imageFetcher struct {
	fail  bool
	calls int
}

func (f *imageFetcher) Fetch(context.Context, string) (capture.Resource, error) {
	f.calls++
	if f.fail {
		return capture.Resource{}, fmt.Errorf("unavailable")
	}
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	return capture.Resource{Body: b}, nil
}
func TestBuildOfflineEPUBWithEditedMetadata(t *testing.T) {
	f := &imageFetcher{}
	s := article.Snapshot{Identity: strings.Repeat("a", 64), Source: "https://example.org/story?a=1&b=2", Metadata: article.Metadata{Title: "Título revisado & leitura", Author: "Autora", Date: "2026-11-05", Language: "pt-BR", Tags: []string{"Artigos", "Ciência"}}, HTML: `<p>Texto &amp; acentos.</p><figure><img src="https://example.org/image.png" alt="Foto"><figcaption>Legenda</figcaption></figure><pre><code>x &lt; 2</code></pre>`}
	b, warnings, err := Build(context.Background(), s, f, 5, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || f.calls != 1 {
		t.Fatal(warnings, f.calls)
	}
	if path := os.Getenv("PTC_EPUB_ARTIFACT"); path != "" {
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	if z.File[0].Name != "mimetype" || z.File[0].Method != zip.Store || len(z.File[0].Extra) != 0 {
		t.Fatal("mimetype must be first and stored")
	}
	image, opf, nav := false, false, false
	for _, entry := range z.File {
		r, _ := entry.Open()
		content, _ := io.ReadAll(r)
		r.Close()
		if strings.HasSuffix(entry.Name, ".png") {
			image = true
		}
		if strings.HasSuffix(entry.Name, ".opf") {
			opf = true
			for _, want := range []string{"Título revisado &amp; leitura", "<dc:source>", "<dc:subject>Ciência</dc:subject>", "2026-11-05", "page-to-calibre:"} {
				if !bytes.Contains(content, []byte(want)) {
					t.Fatalf("missing %s in OPF: %s", want, content)
				}
			}
		}
		if strings.Contains(entry.Name, "nav") {
			nav = true
		}
		if strings.HasSuffix(entry.Name, ".xhtml") || strings.HasSuffix(entry.Name, ".opf") || strings.HasSuffix(entry.Name, ".ncx") {
			d := xml.NewDecoder(bytes.NewReader(content))
			for {
				_, err := d.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("invalid XML %s: %v", entry.Name, err)
				}
			}
		}
		if strings.HasSuffix(entry.Name, "article.xhtml") && bytes.Contains(content, []byte(`src="https://`)) {
			t.Fatal("remote image left in EPUB")
		}
	}
	if !image || !opf || !nav {
		t.Fatal("missing EPUB assets", image, opf, nav)
	}
}
func TestBuildKeepsTextWhenImageUnavailable(t *testing.T) {
	f := &imageFetcher{fail: true}
	b, w, err := Build(context.Background(), article.Snapshot{Source: "https://example.org/story", Identity: strings.Repeat("a", 64), Metadata: article.Metadata{Title: "Text"}, HTML: `<p>Keep this text</p><img src="https://example.org/x" alt="Unavailable">`}, f, 1, 32<<20)
	reader, _ := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, ".opf") {
			r, _ := f.Open()
			content, _ := io.ReadAll(r)
			r.Close()
			if bytes.Contains(content, []byte("dc:creator")) {
				t.Fatal("absent author emitted as empty creator")
			}
		}
	}
	if path := os.Getenv("PTC_EPUB_MISSING_ARTIFACT"); path != "" {
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err != nil || len(b) == 0 || len(w) != 1 {
		t.Fatal(len(b), w, err)
	}
}
