package cmd

import (
	"context"

	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/index"
	"github.com/Malowking/kbgo/internal/logic/retriever"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/gogf/gf/v2/frame/g"
)

// InitAll initializes all components of the application
func init() {
	ctx := context.Background()

	// Validate configuration before initializing components
	g.Log().Info(ctx, "Validating application configuration...")
	err := config.ValidateConfiguration(ctx)
	if err != nil {
		g.Log().Fatalf(ctx, "Configuration validation failed:\n%v", err)
	}

	// Initialize database
	err = dao.InitDB()
	if err != nil {
		g.Log().Fatalf(ctx, "Database connection initialization failed: %v", err)
	}

	// Initialize storage system
	file_store.InitStorage()

	// Initialize vector database
	_, err = service.GetVectorStore()
	if err != nil {
		g.Log().Fatalf(ctx, "Vector store initialization failed: %v", err)
	}

	// Initialize document indexer
	index.InitDocumentIndexer()

	// Initialize retriever configuration
	retriever.InitRetrieverConfig()

	// Initialize chat handler
	chat.InitChat()

	g.Log().Info(ctx, "✓ All components initialized successfully")
}
