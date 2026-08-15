package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerHasNoMarkdownParser(t *testing.T) {
	var found []string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, "internal/config") {
			return err
		}
		b, _ := os.ReadFile(path)
		for _, marker := range []string{"goldmark", "blackfriday", "markdown"} {
			if strings.Contains(strings.ToLower(string(b)), marker) {
				found = append(found, path+":"+marker)
			}
		}
		return nil
	})
	if len(found) != 0 {
		t.Fatalf("plaintext parser references: %v", found)
	}
}

func TestRoutingCiphertextIsOpaqueAndSizeCapped(t *testing.T) {
	b, err := os.ReadFile("object_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "X-Kynotes-Routing-Ciphertext") || !strings.Contains(s, "len(routing) > 1024") {
		t.Fatal("routing ciphertext is not treated as a capped opaque field")
	}
}

func TestRoutingCiphertextIsReturnedInChangeListing(t *testing.T) {
	b, err := os.ReadFile("sync_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "routingCiphertext") {
		t.Fatal("routing ciphertext is absent from changes")
	}
}
