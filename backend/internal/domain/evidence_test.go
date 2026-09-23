package domain

import "testing"

func TestNewEvidence(t *testing.T) {
	article, event := "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "9b2e6c1a-0d4f-4c1e-8a57-2f6d3b9e7a10"

	e, err := NewEvidence(article, event)
	if err != nil || e.ArticleID != article || e.EventID != event {
		t.Fatalf("NewEvidence = %+v, %v", e, err)
	}

	tests := []struct{ name, article, event, field string }{
		{"missing article_id", "", event, "article_id"},
		{"invalid article_id", "abc", event, "article_id"},
		{"missing event_id", article, "", "event_id"},
		{"invalid event_id", article, "not-a-uuid", "event_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEvidence(tt.article, tt.event)
			assertField(t, err, tt.field)
		})
	}
}
