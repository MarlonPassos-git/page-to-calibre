// Package book packages an extracted snapshot and controlled image downloads as EPUB.
package book

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/MarlonPassos-git/page-to-calibre/internal/article"
	"github.com/MarlonPassos-git/page-to-calibre/internal/capture"
	epub "github.com/go-shiori/go-epub"
	dom "golang.org/x/net/html"
)

type Fetcher interface {
	Fetch(context.Context, string) (capture.Resource, error)
}

// Build does not hand URLs to go-epub: all images are fetched through Fetcher.
// Individual missing images produce warnings; no remote image references remain.
func Build(ctx context.Context, s article.Snapshot, fetcher Fetcher, maxImages int, maxImageBytes int64) ([]byte, []string, error) {
	if _, err := capture.ValidateURL(s.Source); err != nil {
		return nil, nil, fmt.Errorf("origem do snapshot inválida: %w", err)
	}
	e, err := epub.NewEpub(s.Metadata.Title)
	if err != nil {
		return nil, nil, err
	}
	e.SetIdentifier("page-to-calibre:" + s.Identity)
	if strings.TrimSpace(s.Metadata.Author) != "" {
		e.SetAuthor(s.Metadata.Author)
	}
	e.SetDescription(s.Metadata.Description)
	lang := s.Metadata.Language
	if lang == "" {
		lang = "und"
	}
	e.SetLang(lang)
	css, err := e.AddCSS("data:text/css;base64,"+base64.StdEncoding.EncodeToString([]byte("img { max-width: 100%; height: auto; } pre { white-space: pre-wrap; overflow-wrap: anywhere; } body { margin: 0; }")), "article.css")
	if err != nil {
		return nil, nil, err
	}
	doc, err := dom.Parse(strings.NewReader(s.HTML))
	if err != nil {
		return nil, nil, err
	}
	warnings := []string{}
	count := 0
	var imageBytes int64
	if maxImageBytes <= 0 {
		maxImageBytes = 32 << 20
	}
	images := map[string]string{}
	article.Walk(doc, func(n *dom.Node) {
		if n.Type != dom.ElementNode || n.Data != "img" {
			return
		}
		src := ""
		alt := "Imagem"
		for _, a := range n.Attr {
			if a.Key == "src" {
				src = a.Val
			}
			if a.Key == "alt" && a.Val != "" {
				alt = a.Val
			}
		}
		local, known := images[src]
		if !known && count >= maxImages {
			warnings = append(warnings, "Limite de imagens atingido; algumas imagens foram omitidas.")
			images[src] = ""
		}
		if !known && count < maxImages {
			count++
			resource, fetchErr := fetcher.Fetch(ctx, src)
			if fetchErr == nil && imageBytes+int64(len(resource.Body)) > maxImageBytes {
				fetchErr = fmt.Errorf("limite agregado de imagens excedido")
			}
			if fetchErr == nil {
				imageBytes += int64(len(resource.Body))
				kind := httpImageType(resource.Body)
				if kind != "" {
					local, fetchErr = e.AddImage("data:image/"+kind+";base64,"+base64.StdEncoding.EncodeToString(resource.Body), fmt.Sprintf("image-%d.%s", count, kind))
				} else {
					fetchErr = fmt.Errorf("formato de imagem não suportado")
				}
			}
			if fetchErr != nil {
				warnings = append(warnings, "Uma imagem não pôde ser incluída.")
			}
			images[src] = local
		}
		if local == "" {
			n.Type = dom.TextNode
			n.Data = "[" + alt + "]"
			n.Attr = nil
			return
		}
		n.Attr = []dom.Attribute{{Key: "src", Val: local}, {Key: "alt", Val: alt}}
	})
	var body bytes.Buffer
	article.Walk(doc, func(n *dom.Node) {
		if n.Type == dom.ElementNode && n.Data == "body" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				writeXHTML(&body, c)
			}
		}
	})
	source := "<p>Fonte: <a href=\"" + html.EscapeString(s.Source) + "\">" + html.EscapeString(s.Source) + "</a></p>"
	if _, err = e.AddSection("<h1>"+html.EscapeString(s.Metadata.Title)+"</h1>"+body.String()+source, s.Metadata.Title, "article.xhtml", css); err != nil {
		return nil, warnings, err
	}
	var out bytes.Buffer
	if _, err = e.WriteTo(&out); err != nil {
		return nil, warnings, err
	}
	result, err := metadata(out.Bytes(), s)
	return result, warnings, err
}

func httpImageType(b []byte) string {
	if len(b) >= 8 && bytes.Equal(b[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return "png"
	}
	if len(b) > 3 && b[0] == 255 && b[1] == 216 && b[2] == 255 {
		return "jpeg"
	}
	if bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")) {
		return "gif"
	}
	return ""
}
func writeXHTML(w io.Writer, n *dom.Node) {
	if n.Type == dom.TextNode {
		_ = xml.EscapeText(w, []byte(n.Data))
		return
	}
	if n.Type != dom.ElementNode {
		return
	}
	fmt.Fprint(w, "<", n.Data)
	for _, a := range n.Attr {
		fmt.Fprint(w, " ", a.Key, "=\"")
		_ = xml.EscapeText(w, []byte(a.Val))
		fmt.Fprint(w, "\"")
	}
	fmt.Fprint(w, ">")
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeXHTML(w, c)
	}
	fmt.Fprint(w, "</", n.Data, ">")
}

// go-epub exposes no setters for source/date/subjects; add only these standard
// Dublin Core elements to its generated OPF, leaving its manifest/spine intact.
func metadata(input []byte, s article.Snapshot) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(input), int64(len(input)))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, f := range reader.File {
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(f.Name, ".opf") {
			extra := "<dc:source>" + html.EscapeString(s.Source) + "</dc:source>"
			if s.OriginalURL != "" && s.OriginalURL != s.Source {
				extra += "<dc:source>" + html.EscapeString(s.OriginalURL) + "</dc:source>"
			}
			if s.Canonical != "" {
				extra += "<dc:relation>" + html.EscapeString(s.Canonical) + "</dc:relation>"
			}
			if s.Metadata.Date != "" {
				extra += "<dc:date>" + html.EscapeString(s.Metadata.Date) + "</dc:date>"
			}
			for _, tag := range s.Metadata.Tags {
				extra += "<dc:subject>" + html.EscapeString(tag) + "</dc:subject>"
			}
			body = bytes.Replace(body, []byte("</metadata>"), []byte(extra+"</metadata>"), 1)
		}
		header := f.FileHeader
		if f.Name == "mimetype" {
			header.Extra = nil
			header.Modified = time.Time{}
		}
		w, err := writer.CreateHeader(&header)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write(body); err != nil {
			return nil, err
		}
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
