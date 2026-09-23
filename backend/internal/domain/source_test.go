package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewSource_Valid(t *testing.T) {
	s, err := NewSource("  Reuters  ", "https://www.reuters.com/", "  Global news organization. ")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if s.Name != "Reuters" {
		t.Errorf("Name = %q, want trimmed %q", s.Name, "Reuters")
	}
	if s.Description != "Global news organization." {
		t.Errorf("Description = %q, want trimmed", s.Description)
	}
}

func TestNewSource_PreservesURLVerbatim(t *testing.T) {
	urls := []string{
		"https://www.reuters.com/",
		"http://example.com",
		"HTTPS://Example.COM/Path?q=1&b=2#frag",
		"https://data.usgs.gov:8443/a/../b",
	}
	for _, raw := range urls {
		s, err := NewSource("Source", raw, "")
		if err != nil {
			t.Errorf("NewSource(%q): %v", raw, err)
			continue
		}
		if s.URL != raw {
			t.Errorf("URL = %q, want verbatim %q", s.URL, raw)
		}
	}
}

func TestNewSource_Invalid(t *testing.T) {
	tests := []struct {
		name, srcName, url, desc, field string
	}{
		{"missing name", "", "https://example.com", "", "name"},
		{"whitespace name", "   ", "https://example.com", "", "name"},
		{"name too long", strings.Repeat("a", MaxSourceNameLength+1), "https://example.com", "", "name"},
		{"missing url", "Source", "", "", "url"},
		{"relative url", "Source", "/news", "", "url"},
		{"no scheme", "Source", "example.com", "", "url"},
		{"ftp scheme", "Source", "ftp://example.com", "", "url"},
		{"javascript scheme", "Source", "javascript:alert(1)", "", "url"},
		{"missing host", "Source", "https://", "", "url"},
		{"leading space", "Source", " https://example.com", "", "url"},
		{"inner space", "Source", "https://example.com/a b", "", "url"},
		{"url too long", "Source", "https://example.com/" + strings.Repeat("a", MaxSourceURLLength), "", "url"},
		{"description too long", "Source", "https://example.com", strings.Repeat("a", MaxSourceDescriptionLength+1), "description"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSource(tt.srcName, tt.url, tt.desc)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if verr.Field != tt.field {
				t.Errorf("Field = %q, want %q", verr.Field, tt.field)
			}
		})
	}
}

func TestNewSource_LimitsCountCharactersNotBytes(t *testing.T) {
	name := strings.Repeat("台", MaxSourceNameLength) // 3 bytes per character
	if _, err := NewSource(name, "https://example.com", ""); err != nil {
		t.Errorf("name of %d multibyte characters rejected: %v", MaxSourceNameLength, err)
	}
}

func TestIsValidID(t *testing.T) {
	valid := []string{"3f2504e0-4f89-41d3-9a0c-0305e82c3301", "3F2504E0-4F89-41D3-9A0C-0305E82C3301"}
	invalid := []string{"", "123", "not-a-uuid", "3f2504e0-4f89-41d3-9a0c-0305e82c330", "3f2504e0-4f89-41d3-9a0c-0305e82c3301x", "g3f2504e-4f89-41d3-9a0c-0305e82c3301"}

	for _, id := range valid {
		if !IsValidID(id) {
			t.Errorf("IsValidID(%q) = false, want true", id)
		}
	}
	for _, id := range invalid {
		if IsValidID(id) {
			t.Errorf("IsValidID(%q) = true, want false", id)
		}
	}
}
