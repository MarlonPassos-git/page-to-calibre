// Package article extracts and sanitizes readable content without fetching resources.
package article

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"strings"
	"unicode/utf8"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/text/language"
)

type Metadata struct {
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Date        string   `json:"date"`
	Tags        []string `json:"tags"`
	Language    string   `json:"language"`
	Site        string   `json:"site"`
	Description string   `json:"description"`
}
type Snapshot struct {
	OriginalURL string   `json:"original_url,omitempty"`
	Canonical   string   `json:"canonical,omitempty"`
	Metadata    Metadata `json:"metadata"`
	Source      string   `json:"source"`
	Identity    string   `json:"identity"`
	HTML        string   `json:"html"`
	Excerpt     string   `json:"excerpt"`
	Warnings    []string `json:"warnings"`
}

func Identity(u *url.URL) (string, string) {
	clone := *u
	clone.Fragment = ""
	clone.Host = strings.ToLower(clone.Host)
	if (clone.Scheme == "https" && clone.Port() == "443") || (clone.Scheme == "http" && clone.Port() == "80") {
		clone.Host = clone.Hostname()
		if strings.Contains(clone.Host, ":") {
			clone.Host = "[" + clone.Host + "]"
		}
	} else if clone.Port() != "" {
		clone.Host = net.JoinHostPort(clone.Hostname(), clone.Port())
	}
	if clone.Path == "" {
		clone.Path = "/"
	}
	q := clone.Query()
	for key := range q {
		if strings.HasPrefix(strings.ToLower(key), "utm_") || key == "fbclid" || key == "gclid" {
			q.Del(key)
		}
	}
	clone.RawQuery = q.Encode()
	hash := sha256.Sum256([]byte(clone.String()))
	return clone.String(), hex.EncodeToString(hash[:])
}

func Extract(body []byte, base *url.URL) (Snapshot, error) {
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return Snapshot{}, err
	}
	passwordForm := false
	canonical := ""
	Walk(document, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			rel, href := "", ""
			for _, a := range n.Attr {
				if a.Key == "rel" {
					rel = a.Val
				}
				if a.Key == "href" {
					href = a.Val
				}
			}
			if strings.EqualFold(rel, "canonical") {
				if c, e := url.Parse(href); e == nil {
					c = base.ResolveReference(c)
					if c.User == nil && (c.Scheme == "http" || c.Scheme == "https") && strings.EqualFold(c.Host, base.Host) {
						canonical = c.String()
					}
				}
			}
		}
		if n.Type == html.ElementNode && n.Data == "input" {
			for _, a := range n.Attr {
				if a.Key == "type" && strings.EqualFold(a.Val, "password") {
					passwordForm = true
				}
			}
		}
	})
	if passwordForm {
		return Snapshot{}, errors.New("página com autenticação; login não é suportado")
	}
	a, err := readability.FromDocument(document, base)
	if err != nil {
		return Snapshot{}, err
	}
	var rendered, text bytes.Buffer
	if err = a.RenderText(&text); err != nil {
		return Snapshot{}, err
	}
	if a.Node == nil || utf8.RuneCountInString(strings.TrimSpace(text.String())) < 200 || strings.TrimSpace(a.Title()) == "" {
		return Snapshot{}, errors.New("página sem artigo legível; pode exigir login ou JavaScript")
	}
	if err = a.RenderHTML(&rendered); err != nil {
		return Snapshot{}, err
	}
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("figure", "figcaption")
	policy.AllowAttrs("src", "alt", "title").OnElements("img")
	clean := policy.Sanitize(rendered.String())
	doc, err := html.Parse(strings.NewReader(clean))
	if err != nil {
		return Snapshot{}, err
	}
	Walk(doc, func(n *html.Node) {
		for i, attr := range n.Attr {
			if attr.Key == "href" || attr.Key == "src" {
				if u, e := url.Parse(attr.Val); e == nil {
					n.Attr[i].Val = base.ResolveReference(u).String()
				}
			}
		}
	})
	clean, err = BodyHTML(doc)
	if err != nil {
		return Snapshot{}, err
	}
	source, id := Identity(base)
	lang := ""
	if tag, e := language.Parse(strings.ReplaceAll(a.Language(), "_", "-")); e == nil && a.Language() != "" {
		lang = tag.String()
	}
	date := ""
	if d, e := a.PublishedTime(); e == nil {
		date = d.Format("2006-01-02")
	}
	excerpt := []rune(strings.TrimSpace(text.String()))
	if len(excerpt) > 400 {
		excerpt = excerpt[:400]
	}
	return Snapshot{Canonical: canonical, OriginalURL: base.String(), Metadata: Metadata{Title: a.Title(), Author: a.Byline(), Date: date, Tags: []string{"Artigos"}, Language: lang, Site: a.SiteName(), Description: a.Excerpt()}, Source: source, Identity: id, HTML: clean, Excerpt: string(excerpt), Warnings: []string{}}, nil
}

func Walk(n *html.Node, visit func(*html.Node)) {
	visit(n)
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		Walk(c, visit)
		c = next
	}
}
func BodyHTML(doc *html.Node) (string, error) {
	var out bytes.Buffer
	var err error
	Walk(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if e := html.Render(&out, c); e != nil {
					err = e
				}
			}
		}
	})
	return out.String(), err
}
