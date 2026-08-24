package app

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"

	"vhome/8v/agconfig"
	"vhome/8v/agent"
	"vhome/8v/llm"
	"vhome/8v/tools"
	"vhome/internal/DB/mysql"
	"vhome/internal/auth"
	"vhome/internal/config"
	"vhome/internal/http/router"
	"vhome/internal/repository"
	"vhome/internal/service"
)

type APP struct {
	Config config.Config
	Engine *gin.Engine
	DB     *sql.DB

	cancelWorkers context.CancelFunc
	workers       sync.WaitGroup
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
	mealService, err := service.NewMealService(repo)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create meal service: %w", err)
	}
	expenseService, err := service.NewExpenseService(repo)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create expense service: %w", err)
	}
	memoService, err := service.NewMemoService(repo)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create memo service: %w", err)
	}
	secretBox, err := auth.NewSecretBox(cfg.Security.SecretEncryptionKey)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create notification secret box: %w", err)
	}
	notificationService, err := service.NewNotificationService(repo, secretBox)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create notification service: %w", err)
	}
	calendarSyncService, err := service.NewCalendarSyncService(repo)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create calendar sync service: %w", err)
	}
	dashboardService := service.NewDashboardService(repo, pantryService, expenseService, memoService)
	agentConfig := agconfig.Load()
	llmClient, err := llm.NewClient(agentConfig)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create agent llm client: %w", err)
	}
	toolRegistry := tools.NewRegistry("", tools.Options{
		Pantry:  pantryService,
		Expense: expenseService,
	})
	agentFactory := func() *agent.Agent {
		return agent.New(llmClient, toolRegistry)
	}

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
		mealService,
		expenseService,
		memoService,
		notificationService,
		calendarSyncService,
		dashboardService,
		cfg.Auth,
		agentFactory,
	)

	runtimeContext, cancelWorkers := context.WithCancel(ctx)
	application := &APP{
		DB:            db,
		Engine:        engine,
		Config:        cfg,
		cancelWorkers: cancelWorkers,
	}
	application.startMemoWorkers(runtimeContext, memoService, notificationService, calendarSyncService)
	return application, nil
}

func (a *APP) Close() error {
	if a.cancelWorkers != nil {
		a.cancelWorkers()
		a.workers.Wait()
	}
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
