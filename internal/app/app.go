package app

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gin-gonic/gin"

	"vhome/internal/DB/mysql"
	"vhome/internal/config"
	"vhome/internal/http/router"
	"vhome/internal/repository"
	"vhome/internal/service"
)

type APP struct {
	Config config.Config
	Engine *gin.Engine
	DB     *sql.DB
}

func New(ctx context.Context, envFile string) (*APP, error) {
	cfg, err := config.LoadConfig(envFile)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to load config: %w",
			err,
		)
	}

	databaseCtx, cancel := context.WithTimeout(
		ctx,
		cfg.DB.ConnectTimeout,
	)
	defer cancel()

	db, err := mysql.Open(databaseCtx, cfg.DB)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to open database: %w",
			err,
		)
	}

	repo := repository.New(db)

	identityService, err := service.NewIdentityService(repo, cfg.Auth.SessionTTL)
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf(
			"create identity service: %w",
			err,
		)
	}
	pantryService := service.NewPantryService(repo)

	setGinMode(cfg.App.Env)

	engine := gin.Default()
	if err := engine.SetTrustedProxies(nil); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("disable untrusted proxy headers: %w", err)
	}
	engine.Static("/uploads", "data/uploads")

	router.Register(
		engine,
		identityService,
		pantryService,
		cfg.Auth,
	)

	return &APP{
		DB:     db,
		Engine: engine,
		Config: cfg,
	}, nil
}

func (a *APP) Close() error {
	if a.DB == nil {
		return nil
	}

	if err := a.DB.Close(); err != nil {
		return err
	}

	return nil
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
