// Package calibre integrates exclusively through the official calibredb CLI.
package calibre

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Runner func(context.Context, ...string) ([]byte, error)
type Config struct {
	PublicURL      string
	URL            string
	Library        string
	Username       string
	PasswordFile   string
	TimeoutSeconds int
}
type Book struct {
	ID          int               `json:"id"`
	Title       string            `json:"title"`
	Identifiers map[string]string `json:"identifiers"`
	Formats     []string          `json:"formats"`
	URL         string            `json:"url"`
	Duplicate   bool              `json:"duplicate"`
}
type Client struct {
	config Config
	run    Runner
	mu     sync.Mutex
}

var ErrUncertain = errors.New("resultado incerto: confirme a operação antes de reenviar")
var identityPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func New(config Config, run Runner) (*Client, error) {
	u, err := url.Parse(config.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("CALIBRE_URL: esperado endereço HTTP(S) sem credenciais, query ou fragmento")
	}
	if config.PublicURL == "" {
		config.PublicURL = config.URL
	}
	public, parseErr := url.Parse(config.PublicURL)
	if parseErr != nil || public.Hostname() == "" || public.User != nil || public.RawQuery != "" || public.Fragment != "" || (public.Scheme != "http" && public.Scheme != "https") {
		return nil, errors.New("CALIBRE_PUBLIC_URL: esperado endereço HTTP(S) sem credenciais, query ou fragmento")
	}
	if config.Library == "" || config.Username == "" || !strings.HasPrefix(config.PasswordFile, "/") || strings.Contains(config.PasswordFile, ">") {
		return nil, errors.New("Calibre: biblioteca, usuário e caminho absoluto de secret são obrigatórios")
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 60
	}
	if run == nil {
		run = func(ctx context.Context, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, "calibredb", args...).Output()
		}
	}
	return &Client{config: config, run: run}, nil
}
func (c *Client) args(command string, options ...string) []string {
	u, _ := url.Parse(c.config.URL)
	u.Fragment = c.config.Library
	args := []string{command, "--with-library", u.String(), "--username", c.config.Username, "--password", "<f:" + c.config.PasswordFile + ">", "--timeout", strconv.Itoa(c.config.TimeoutSeconds)}
	return append(args, options...)
}

// Find confirms the origin identifier AND EPUB format. A match without EPUB is
// an error, because importing must never modify an existing origin record.
func (c *Client) Find(ctx context.Context, identity string) (*Book, error) {
	if !identityPattern.MatchString(identity) {
		return nil, errors.New("identity: esperado SHA-256 hexadecimal")
	}
	output, err := c.run(ctx, c.args("list", "--for-machine", "--fields", "id,title,identifiers,formats", "--search", `identifiers:"=page-to-calibre:`+identity+`"`)...)
	if err != nil {
		return nil, errors.New("Calibre indisponível, autenticação recusada ou biblioteca inacessível")
	}
	var books []Book
	if err = json.Unmarshal(output, &books); err != nil {
		return nil, errors.New("resposta inválida de calibredb")
	}
	for _, b := range books {
		if b.Identifiers["page-to-calibre"] != identity {
			continue
		}
		found := false
		for _, f := range b.Formats {
			if strings.EqualFold(f, "EPUB") {
				found = true
			}
		}
		if !found {
			return nil, errors.New("item existente sem EPUB; intervenção necessária")
		}
		u, _ := url.Parse(c.config.PublicURL)
		u.Path = strings.TrimSuffix(u.Path, "/") + "/"
		u.Fragment = url.Values{"book_id": {strconv.Itoa(b.ID)}, "library_id": {c.config.Library}, "panel": {"book_details"}}.Encode()
		b.URL = u.String()
		return &b, nil
	}
	return nil, nil
}

// Ensure serializes this instance and reconciles after EVERY attempted write,
// including process failures; it never retries an ambiguous add itself.
func (c *Client) Ensure(ctx context.Context, identity, path string) (Book, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	existing, err := c.Find(ctx, identity)
	if err != nil {
		return Book{}, err
	}
	if existing != nil {
		existing.Duplicate = true
		return *existing, nil
	}
	_, writeErr := c.run(ctx, c.args("add", "--duplicates", "--identifier", "page-to-calibre:"+identity, path)...)
	found, readErr := c.Find(ctx, identity)
	if readErr != nil || found == nil {
		return Book{}, fmt.Errorf("%w", ErrUncertain)
	}
	_ = writeErr // A lost CLI response does not override confirmed persistent state.
	return *found, nil
}
