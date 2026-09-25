package web

import (
	"strings"
	"testing"
)

func TestMarkdownHTMLBold(t *testing.T) {
	got := string(Markdown("**Pasta.**"))
	if !strings.Contains(got, "<strong>Pasta.</strong>") {
		t.Fatalf("got %q, want strong", got)
	}
}

func TestMarkdownHTMLList(t *testing.T) {
	got := string(Markdown("- one\n- two"))
	if !strings.Contains(got, "<li>") || !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Fatalf("got %q, want list", got)
	}
}

func TestMarkdownHTMLFencedCode(t *testing.T) {
	got := string(Markdown("```\nhello\n```"))
	if !strings.Contains(got, "<pre>") || !strings.Contains(got, "<code>") || !strings.Contains(got, "hello") {
		t.Fatalf("got %q, want fenced code", got)
	}
}

func TestMarkdownHTMLEmpty(t *testing.T) {
	if got := string(Markdown("")); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestMarkdownHTMLOmitsRawScript(t *testing.T) {
	got := string(Markdown("<script>alert(1)</script>"))
	if strings.Contains(got, "<script>") {
		t.Fatalf("got %q, want no script tag", got)
	}
}
