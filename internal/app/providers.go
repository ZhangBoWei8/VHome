package app

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gin-gonic/gin"

	"vhome/internal/DB/mysql"
	"vhome/internal/auth"
	"vhome/internal/config"
	"vhome/internal/http/handler"
	"vhome/internal/http/middleware"
	"vhome/internal/http/router"
	"vhome/internal/service"
)

// Workers holds the services the background goroutines need. Bundling them in
// one struct keeps them reachable after the injector has built the graph.
type Workers struct {
	Memo         *service.MemoService
	Notification *service.NotificationService
	Calendar     *service.CalendarSyncService
}

// provideDB returns the pool together with a cleanup function. Wire calls that
// cleanup automatically when any later provider fails, which replaces the
// `_ = db.Close()` that used to be repeated after every constructor.
func provideDB(ctx context.Context, cfg config.DBConfig) (*sql.DB, func(), error) {
	databaseCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	db, err := mysql.Open(databaseCtx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	return db, func() { _ = db.Close() }, nil
}

// provideSessionTTL and provideEncryptionKey adapt configuration fields to the
// named types their consumers declare, so no bare string or duration ever
// enters the object graph.
func provideSessionTTL(cfg config.AuthConfig) service.SessionTTL {
	return service.SessionTTL(cfg.SessionTTL)
}

func provideEncryptionKey(cfg config.SecurityConfig) auth.EncryptionKey {
	return auth.EncryptionKey(cfg.SecretEncryptionKey)
}

// provideEngine wraps router.Register, which returns nothing and therefore
// cannot be a provider on its own.
func provideEngine(appConfig config.AppConfig, storage config.StorageConfig, handlers handler.Handlers, guards middleware.Guards) (*gin.Engine, error) {
	setGinMode(appConfig.Env)

	engine := gin.Default()
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("disable untrusted proxy headers: %w", err)
	}
	engine.Static("/uploads", storage.UploadDir)

	router.Register(engine, handlers, guards)

	return engine, nil
}

// newAPP is written by hand rather than generated with wire.Struct because APP
// carries unexported lifecycle fields that wire cannot set.
func newAPP(cfg config.Config, engine *gin.Engine, db *sql.DB, workers Workers) *APP {
	return &APP{
		Config:  cfg,
		Engine:  engine,
		DB:      db,
		workers: workers,
	}
}

func setGinMode(env string) {
	switch env {
	case "production":
		gin.SetMode(gin.ReleaseMode)

	case "development":
		gin.SetMode(gin.DebugMode)

	default:
		gin.SetMode(gin.TestMode)
	}
}
