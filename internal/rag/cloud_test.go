package rag

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewChatRoutesCloudModelsOnlyWithAKey(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "")
	cases := []struct {
		name, model, key   string
		wantURL, wantModel string
		wantCloud          bool
	}{
		{"dash suffix with key", "gpt-oss:120b-cloud", " k ", CloudBaseURL, "gpt-oss:120b", true},
		{"colon tag with key", "glm-4.6:cloud", "k", CloudBaseURL, "glm-4.6", true},
		{"local model with key", "llama3.2", "k", DefaultBaseURL, "llama3.2", false},
		{"cloud model without key", "gpt-oss:120b-cloud", "", DefaultBaseURL, "gpt-oss:120b-cloud", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := NewChat(DefaultBaseURL, c.model, c.key)
			if o.BaseURL != c.wantURL || o.ChatModel != c.wantModel || o.Cloud() != c.wantCloud {
				t.Fatalf("got url=%q model=%q cloud=%v", o.BaseURL, o.ChatModel, o.Cloud())
			}
			if !c.wantCloud && o.APIKey != "" {
				t.Fatal("a local server must never receive the key")
			}
		})
	}

	t.Setenv("OLLAMA_API_KEY", "from-env")
	if o := NewChat("", "gpt-oss:120b-cloud", ""); o.APIKey != "from-env" {
		t.Fatalf("blank key should fall back to OLLAMA_API_KEY, got %q", o.APIKey)
	}
}

func TestRequestsCarryBearerOnlyWhenKeyed(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	t.Cleanup(srv.Close)

	o := NewOllama(srv.URL, "", "")
	o.Available(context.Background())
	if got != "" {
		t.Fatalf("keyless request sent %q", got)
	}
	o.APIKey = "secret"
	o.Available(context.Background())
	if got != "Bearer secret" {
		t.Fatalf("keyed request sent %q", got)
	}
}
