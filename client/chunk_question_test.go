package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The generated-question delete lives under /chunks/by-id/{id}/questions. The
// SDK used to send /chunks/{id}/delete-question, which the server routes to
// the delete-a-chunk handler (/chunks/:knowledge_id/:id) instead.
func TestDeleteGeneratedQuestionUsesTheQuestionsRoute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/chunks/by-id/c-1/questions" {
			t.Fatalf("got %s %s, want DELETE /api/v1/chunks/by-id/c-1/questions", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["question_id"] != "q-1" {
			t.Fatalf("body = %v (%v), want question_id q-1", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("sk-test"))
	if err := c.DeleteGeneratedQuestion(context.Background(), "c-1", "q-1"); err != nil {
		t.Fatal(err)
	}
}
