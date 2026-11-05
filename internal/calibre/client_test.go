package calibre

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

const testIdentity = "400de2d10ca05e091c975556d2705f2006801bb0b6e671d7c782e4202c0c4eca"

func config() Config {
	return Config{URL: "http://127.0.0.1:8080", Library: "library", Username: "writer", PasswordFile: "/tmp/password"}
}
func record(title string) []byte {
	return []byte(fmt.Sprintf(`[{"id":1,"title":%q,"identifiers":{"page-to-calibre":%q},"formats":["EPUB"]}]`, title, testIdentity))
}
func TestEnsureReconcilesLostAddResponse(t *testing.T) {
	added := false
	adds := 0
	c, _ := New(config(), func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "add" {
			added = true
			adds++
			return nil, errors.New("lost response")
		}
		if added {
			return record("Confirmed"), nil
		}
		return []byte("[]"), nil
	})
	b, err := c.Ensure(context.Background(), testIdentity, "/tmp/book.epub")
	if err != nil || b.ID != 1 || b.Duplicate {
		t.Fatal(b, err)
	}
	b, err = c.Ensure(context.Background(), testIdentity, "/tmp/book.epub")
	if err != nil || !b.Duplicate || adds != 1 {
		t.Fatal(b, err, adds)
	}
}
func TestConcurrentEnsureAddsOnce(t *testing.T) {
	added := false
	adds := 0
	c, _ := New(config(), func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "add" {
			adds++
			added = true
			return nil, nil
		}
		if added {
			return record("Confirmed"), nil
		}
		return []byte("[]"), nil
	})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := c.Ensure(context.Background(), testIdentity, "/tmp/book.epub"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if adds != 1 {
		t.Fatal(adds)
	}
}
func TestUnconfirmedAddIsUncertain(t *testing.T) {
	c, _ := New(config(), func(context.Context, ...string) ([]byte, error) { return []byte("[]"), nil })
	_, err := c.Ensure(context.Background(), testIdentity, "/tmp/book.epub")
	if !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
}
func TestCalibreContract(t *testing.T) {
	container := os.Getenv("PTC_CONTRACT_CONTAINER")
	if container == "" {
		t.Skip("set PTC_CONTRACT_CONTAINER for isolated real Calibre")
	}
	runner := func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "docker", append([]string{"exec", container, "calibredb"}, args...)...).Output()
	}
	c, err := New(config(), runner)
	if err != nil {
		t.Fatal(err)
	}
	contractID := strings.Repeat("b", 64)
	first, err := c.Ensure(context.Background(), contractID, "/tmp/probe.epub")
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Ensure(context.Background(), contractID, "/tmp/probe.epub")
	if err != nil || !second.Duplicate || second.ID != first.ID {
		t.Fatal(first, second, err)
	}
	link, _ := url.Parse(second.URL)
	fragment, _ := url.ParseQuery(link.Fragment)
	if fragment.Get("book_id") != fmt.Sprint(second.ID) || fragment.Get("panel") != "book_details" || fragment.Get("library_id") != "library" {
		t.Fatal(second.URL)
	}
	bad := config()
	bad.Library = "nonexistent"
	bc, _ := New(bad, runner)
	if _, err = bc.Find(context.Background(), testIdentity); err == nil {
		t.Fatal("wrong library accepted")
	}
}

func TestCalibreContractPermissions(t *testing.T) {
	container := os.Getenv("PTC_CONTRACT_CONTAINER")
	if container == "" {
		t.Skip("requires isolated Calibre")
	}
	runner := func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "docker", append([]string{"exec", container, "calibredb"}, args...)...).Output()
	}
	bad := config()
	bad.PasswordFile = "/tmp/wrong-password"
	c, _ := New(bad, runner)
	if _, err := c.Find(context.Background(), testIdentity); err == nil {
		t.Fatal("invalid password accepted")
	}
	readonly := config()
	readonly.Username = "reader"
	c, _ = New(readonly, runner)
	if _, err := c.Ensure(context.Background(), strings.Repeat("c", 64), "/tmp/probe.epub"); !errors.Is(err, ErrUncertain) {
		t.Fatal("write refusal must not report success", err)
	}
}

func TestResultUsesPublicURLAndContentServerFragmentRouting(t *testing.T) {
	settings := config()
	settings.PublicURL = "https://books.example.org/calibre"
	client, err := New(settings, func(context.Context, ...string) ([]byte, error) { return record("Public link"), nil })
	if err != nil {
		t.Fatal(err)
	}
	book, err := client.Find(context.Background(), testIdentity)
	if err != nil {
		t.Fatal(err)
	}
	link, _ := url.Parse(book.URL)
	fragment, _ := url.ParseQuery(link.Fragment)
	if link.Host != "books.example.org" || link.Path != "/calibre/" || fragment.Get("book_id") != "1" || fragment.Get("panel") != "book_details" {
		t.Fatal(book.URL)
	}
}
