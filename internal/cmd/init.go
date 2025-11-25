package cmd

import (
	"context"

	"github.com/Malowking/kbgo/core/indexer/file_store"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/index"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/gogf/gf/v2/frame/g"
)

// InitAll initializes all components of the application
func init() {
	// Initialize vector database
	index.InitVectorDatabase()
	// Initialize storage system
	file_store.InitStorage()
	// Initialize database
	err := dao.InitDB()
	if err != nil {
		g.Log().Fatal(context.Background(), "database connection not initialized")
	}
	// Initialize retriever configuration
	rag.InitRetrieverConfig()
	// Initialize chat handler
	chat.InitChat()
}
