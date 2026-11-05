package article

import (
	"net/url"
	"strings"
	"testing"
)

func TestExtractMetadataAndSafeContent(t *testing.T) {
	u, _ := url.Parse("https://example.org/articles/item?utm_source=test&edition=2#part")
	body := `<html lang="pt-BR"><head><title>Artigo sobre leitura</title><link rel="canonical" href="/canonical"><meta name="author" content="Autora Exemplo"><meta property="article:published_time" content="2026-10-01T12:00:00Z"></head><body><nav>Menu</nav><article><h1>Artigo sobre leitura</h1>` + strings.Repeat(`<p>Leitura com atenção preserva os detalhes, acentos e ideias. Este parágrafo fornece conteúdo legível para a extração do artigo.</p>`, 8) + `<figure><img src="../photo.png" onerror="alert(1)" alt="Foto"><figcaption>Legenda</figcaption></figure><pre><code>fmt.Println(&quot;olá&quot;)</code></pre><script>alert(1)</script><a href="./outro">Referência</a></article></body></html>`
	s, err := Extract([]byte(body), u)
	if err != nil {
		t.Fatal(err)
	}
	if s.Metadata.Title != "Artigo sobre leitura" || s.Metadata.Author != "Autora Exemplo" || s.Metadata.Date != "2026-10-01" {
		t.Fatalf("metadata: %+v", s.Metadata)
	}
	for _, want := range []string{"https://example.org/photo.png", "https://example.org/articles/outro", "<code>", "Legenda"} {
		if !strings.Contains(s.HTML, want) {
			t.Fatalf("missing %s in %s", want, s.HTML)
		}
	}
	for _, bad := range []string{"<script", "onerror=", "alert(1)"} {
		if strings.Contains(s.HTML, bad) {
			t.Fatal("unsafe content", bad)
		}
	}
	if s.Canonical != "https://example.org/canonical" {
		t.Fatal("canonical not captured", s.Canonical)
	}
	if s.Source != "https://example.org/articles/item?edition=2" {
		t.Fatal(s.Source)
	}
}
func TestRejectEmptyLoginOrErrorPage(t *testing.T) {
	u, _ := url.Parse("https://example.org/")
	for _, body := range []string{"", `<html><title>Login</title><body><form><input type="password"></form></body></html>`, `<html><title>Forbidden</title><body>Access denied</body></html>`} {
		if _, err := Extract([]byte(body), u); err == nil {
			t.Fatal("accepted non-article")
		}
	}
}
func TestIdentityPreservesMeaningfulQuery(t *testing.T) {
	u, _ := url.Parse("https://example.org/a?id=1&utm_campaign=x#heading")
	v, _ := url.Parse("https://example.org/a?id=1")
	_, a := Identity(u)
	_, b := Identity(v)
	if a != b {
		t.Fatal("tracking affected identity")
	}
	v.RawQuery = "id=2"
	_, b = Identity(v)
	if a == b {
		t.Fatal("meaningful query removed")
	}
}

func TestIdentityNormalizesDefaultPortsAndEmptyPath(t *testing.T) {
	u, _ := url.Parse("https://EXAMPLE.org:443")
	v, _ := url.Parse("https://example.org/")
	_, a := Identity(u)
	_, b := Identity(v)
	if a != b {
		t.Fatal("default port affected identity")
	}
}
