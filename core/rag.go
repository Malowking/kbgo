package core

import (
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/cloudwego/eino/components/model"
)

// Rag represents the core RAG (Retrieval-Augmented Generation) service
// It provides methods to build retrievers for RAG functionality
type Rag struct {
	VectorStore vector_store.VectorStore
	cm          model.BaseChatModel
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

// GetChatModel returns the chat model
func (r *Rag) GetChatModel() model.BaseChatModel {
	return r.cm
}
