package security

import (
	"os"
	"strings"
	"testing"
)

func TestAdminHTMLDoesNotUseUnsafeDynamicRendering(t *testing.T) {
	b, err := os.ReadFile("../../web/admin.html")
	if err != nil {
		t.Fatalf("read admin.html: %v", err)
	}
	html := string(b)

	if strings.Contains(html, ".innerHTML") {
		t.Fatalf("admin.html must not use innerHTML for dynamic content")
	}
	if strings.Contains(html, "onclick=") {
		t.Fatalf("admin.html must not use inline event handlers")
	}
}

func TestAdminHTMLUsesExternalScripts(t *testing.T) {
	b, err := os.ReadFile("../../web/admin.html")
	if err != nil {
		t.Fatalf("read admin.html: %v", err)
	}
	html := string(b)

	if strings.Contains(html, "<script>") {
		t.Fatalf("admin.html must not include inline script blocks")
	}
	if !strings.Contains(html, `<script src="/admin.js" defer></script>`) {
		t.Fatalf("admin.html must load admin.js as an external script")
	}
}

func TestSecurityHeadersDisallowInlineScripts(t *testing.T) {
	b, err := os.ReadFile("../../cmd/server/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	mainGo := string(b)

	if strings.Contains(mainGo, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("content security policy must not allow inline scripts")
	}
}
