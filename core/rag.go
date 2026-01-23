package core

import (
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/vector_store"
)

// Rag represents the core RAG (Retrieval-Augmented Generation) service
type Rag struct {
	VectorStore vector_store.VectorStore
	conf        *config.Config
}

// GetConfig returns the configuration
func (r *Rag) GetConfig() *config.Config {
	return r.conf
}

// GetRetrieverConfig returns retriever-specific configuration
func (r *Rag) GetRetrieverConfig() *config.RetrieverConfig {
	return &config.RetrieverConfig{
		RetrieverConfigBase: config.RetrieverConfigBase{
			MetricType:     r.conf.MetricType,
			APIKey:         r.conf.APIKey,
			BaseURL:        r.conf.BaseURL,
			EmbeddingModel: r.conf.EmbeddingModel,
		},
		VectorStore: r.VectorStore,
	}
}
