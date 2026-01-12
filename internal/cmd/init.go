package cmd

import (
	"github.com/Malowking/kbgo/core/model"
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/core/vector_store"
	internalCache "github.com/Malowking/kbgo/internal/cache"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/logic/index"
	"github.com/Malowking/kbgo/internal/logic/retriever"
	"github.com/gogf/gf/v2/frame/g"
)

// InitAll initializes all components of the application
func init() {
	ctx := gctx.New()

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

	// Initialize Redis cache
	g.Log().Info(ctx, "Initializing Redis cache...")
	err = cache.InitRedis(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "Redis initialization failed (non-fatal): %v", err)
		g.Log().Warning(ctx, "Agent preset caching will be disabled")
	} else {
		g.Log().Info(ctx, "✓ Redis cache initialized successfully")

		// Initialize message cache layer
		g.Log().Info(ctx, "Initializing message cache layer...")
		err = internalCache.InitMessageCache(ctx)
		if err != nil {
			g.Log().Warningf(ctx, "Message cache layer initialization failed (non-fatal): %v", err)
		} else {
			g.Log().Info(ctx, "✓ Message cache layer initialized successfully")
		}

		// Initialize MCP call log cache layer
		g.Log().Info(ctx, "Initializing MCP call log cache layer...")
		err = internalCache.InitMCPCallLogCache(ctx)
		if err != nil {
			g.Log().Warningf(ctx, "MCP call log cache layer initialization failed (non-fatal): %v", err)
		} else {
			g.Log().Info(ctx, "✓ MCP call log cache layer initialized successfully")
		}
	}

	// Initialize storage system
	file_store.InitStorage()

	// Initialize vector database
	_, err = vector_store.GetVectorStore()
	if err != nil {
		g.Log().Fatalf(ctx, "Vector store initialization failed: %v", err)
	}

	// Initialize model registry from database
	g.Log().Info(ctx, "Initializing model registry...")
	err = model.Registry.Reload(ctx, dao.GetDB())
	if err != nil {
		g.Log().Warningf(ctx, "Model registry initialization failed (non-fatal): %v", err)
		g.Log().Warning(ctx, "You can add models to model table and call /v1/model/reload to load them")
	} else {
		g.Log().Infof(ctx, "✓ Model registry initialized successfully with %d models", model.Registry.Count())
	}

	// Initialize document indexer
	index.InitDocumentIndexer()

	// Initialize retriever configuration
	retriever.InitRetrieverConfig()

	// Initialize chat history manager
	chat.InitHistory()

	g.Log().Info(ctx, "✓ All components initialized successfully")
}
