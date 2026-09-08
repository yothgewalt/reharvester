// Package rag holds the parts that need a model: the sentence encoder behind
// tier T3, and the local generation behind wiki synthesis and question
// answering. Everything here is optional by construction — an unreachable
// Ollama drops the system a rung rather than breaking it.
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yothgewalt/reharvester/internal/index"
)

const (
	DefaultBaseURL   = "http://localhost:11434"
	DefaultEmbedding = "all-minilm" // 384-dimensional MiniLM, about 45 MB
	DefaultChatModel = "llama3.2"
	embedDim         = 384
)

// Ollama talks to a local model server. It is the only network client in the
// system after harvesting, and it never leaves the machine.
type Ollama struct {
	BaseURL    string
	EmbedModel string
	ChatModel  string
	HTTP       *http.Client
	Batch      int
}

func NewOllama(baseURL, embedModel, chatModel string) *Ollama {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if embedModel == "" {
		embedModel = DefaultEmbedding
	}
	if chatModel == "" {
		chatModel = DefaultChatModel
	}
	return &Ollama{
		BaseURL:    baseURL,
		EmbedModel: embedModel,
		ChatModel:  chatModel,
		HTTP:       &http.Client{Timeout: 5 * time.Minute},
		Batch:      64,
	}
}

func (o *Ollama) Dim() int { return embedDim }

// Available reports whether the server answers at all. The health endpoint uses
// it under a short deadline, so it must not be allowed to hang.
func (o *Ollama) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	res, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}

// HasModel reports whether a specific model has been pulled. A reachable server
// with no encoder pulled is still tier T2, so the two checks are distinct.
func (o *Ollama) HasModel(ctx context.Context, name string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	res, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	var body struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if json.NewDecoder(res.Body).Decode(&body) != nil {
		return false
	}
	for _, m := range body.Models {
		if m.Name == name || m.Model == name ||
			m.Name == name+":latest" || m.Model == name+":latest" {
			return true
		}
	}
	return false
}

// models lists what the server actually has pulled.
func (o *Ollama) models(ctx context.Context) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil
	}
	res, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if json.NewDecoder(res.Body).Decode(&body) != nil {
		return nil
	}
	out := make([]string, 0, len(body.Models))
	for _, m := range body.Models {
		out = append(out, m.Name)
	}
	return out
}

// chatModelPreference lists model families known to produce prose quickly on a
// laptop, in descending preference. Matching is by prefix so a tag like
// "llama3.2:3b" counts.
var chatModelPreference = []string{
	"llama3.2", "llama3.1", "llama3", "qwen2.5", "qwen3",
	"mistral", "gemma2", "gemma3", "phi4", "phi3", "smollm2",
}

// ResolveChatModel keeps the configured model if it is pulled, and otherwise
// adopts the best pulled model from a known-good list.
//
// It deliberately does NOT fall back to "any model that is not the encoder".
// Doing so once picked up a 7B theorem-proving model, which is reachable,
// answers the tags endpoint, and then fails to produce a sentence inside four
// minutes — so the health endpoint claimed generation worked while every wiki
// page silently fell back. A model that is present is not evidence that it can
// write prose.
func (o *Ollama) ResolveChatModel(ctx context.Context) bool {
	pulled := o.models(ctx)
	has := func(name string) bool {
		for _, m := range pulled {
			if m == name || m == name+":latest" {
				return true
			}
		}
		return false
	}
	if has(o.ChatModel) {
		return true
	}
	for _, want := range chatModelPreference {
		for _, m := range pulled {
			if strings.HasPrefix(strings.ToLower(m), want) {
				o.ChatModel = m
				return true
			}
		}
	}
	return false
}

// SuggestChatModel names what to pull when nothing suitable is installed.
func SuggestChatModel() string { return chatModelPreference[0] }

// CanGenerate reports whether prose generation will actually work, which is
// stronger than the server merely answering.
func (o *Ollama) CanGenerate(ctx context.Context) bool {
	return o != nil && o.Available(ctx) && o.ResolveChatModel(ctx)
}

// Embed encodes texts into unit-length vectors, batching and running batches in
// parallel. Encoding the corpus is by far the most expensive stage of a build,
// which is why the result is persisted and the tier treated as optional.
func (o *Ollama) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	batch := o.Batch
	if batch < 1 {
		batch = 32
	}
	type job struct {
		start int
		texts []string
	}
	var jobs []job
	for i := 0; i < len(texts); i += batch {
		end := min(i+batch, len(texts))
		jobs = append(jobs, job{start: i, texts: texts[i:end]})
	}
	out := make([][]float32, len(texts))
	workers := min(runtime.GOMAXPROCS(0), 4) // the server is the bottleneck, not us
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	ch := make(chan job)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				vecs, err := o.embedBatch(ctx, j.texts)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				for i, v := range vecs {
					index.Normalize(v)
					out[j.start+i] = v
				}
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func (o *Ollama) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"model": o.EmbedModel, "input": texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := o.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed: %s", res.Status)
	}
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embed: got %d vectors for %d texts", len(out.Embeddings), len(texts))
	}
	return out.Embeddings, nil
}

// GenerateStream asks the local model for prose and delivers it as it is
// written, calling onToken for each chunk and returning the whole text.
//
// Generation runs at roughly the model's decode rate — tens of milliseconds
// per token on a laptop CPU — so a few hundred tokens is tens of seconds. That
// cost is unavoidable, but making the caller wait for all of it is not: the
// streaming form exists so a reader sees the first words in about a second.
// Prefer it for anything a person is watching, and keep Generate for callers
// that only want the finished string.
//
// onToken runs on this goroutine, in order, and must not block.
func (o *Ollama) GenerateStream(ctx context.Context, system, prompt string, onToken func(string)) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":  o.ChatModel,
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := o.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama chat: %s", res.Status)
	}

	// The streaming endpoint answers with newline-delimited JSON objects rather
	// than one document, so decode in a loop until it reports done.
	var sb strings.Builder
	dec := json.NewDecoder(res.Body)
	for {
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if err := dec.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			// A partial answer is worth more than nothing: return what arrived
			// alongside the error and let the caller decide.
			return sb.String(), err
		}
		if chunk.Error != "" {
			return sb.String(), fmt.Errorf("ollama chat: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			sb.WriteString(chunk.Message.Content)
			if onToken != nil {
				onToken(chunk.Message.Content)
			}
		}
		if chunk.Done {
			break
		}
	}
	return sb.String(), nil
}

// Generate asks the local model for prose. Callers must treat an error as
// "this feature is unavailable right now", never as a failure of the request
// that triggered it.
func (o *Ollama) Generate(ctx context.Context, system, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":  o.ChatModel,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := o.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama chat: %s", res.Status)
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Message.Content, nil
}
