package rag

import (
	"context"
	"log"

	"github.com/yothgewalt/reharvester/internal/index"
)

// ProbeEmbedder returns the T3 encoder, or a nil index.Embedder when no model
// server answers or the model is not pulled. A nil encoder is the T2 rung of
// the capability ladder, not an error — callers should run without dense
// retrieval rather than fail.
func ProbeEmbedder(ctx context.Context, baseURL, model string) index.Embedder {
	o := NewOllama(baseURL, model, "")
	if !o.Available(ctx) {
		log.Printf("encoder: no model server at %s — running at tier T2", baseURL)
		return nil
	}
	if !o.HasModel(ctx, model) {
		log.Printf("encoder: model %q not pulled (try: ollama pull %s) — running at tier T2", model, model)
		return nil
	}
	return o
}
