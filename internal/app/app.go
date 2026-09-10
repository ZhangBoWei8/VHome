package app

import (
	"context"
	"database/sql"
	"sync"

	"github.com/gin-gonic/gin"

	"vhome/internal/config"
)

type APP struct {
	Config config.Config
	Engine *gin.Engine
	DB     *sql.DB

	workers       Workers
	cancelWorkers context.CancelFunc
	workerGroup   sync.WaitGroup
}

// New builds the application. Every dependency is resolved by the generated
// injector; this function only owns what wire deliberately does not: starting
// and stopping the background workers.
//
// The returned cleanup releases the resources the injector acquired (currently
// the database pool) and must be called after Close.
func New(ctx context.Context, envFile config.EnvFile) (*APP, func(), error) {
	application, cleanup, err := InitializeAPP(ctx, envFile)
	if err != nil {
		return nil, nil, err
	}

	runtimeContext, cancelWorkers := context.WithCancel(ctx)
	application.cancelWorkers = cancelWorkers
	application.startMemoWorkers(runtimeContext)

	return application, cleanup, nil
}

// Close stops the background workers. The database pool is closed by the
// cleanup function returned from New.
func (a *APP) Close() error {
	if a.cancelWorkers != nil {
		a.cancelWorkers()
		a.workerGroup.Wait()
	}

	return nil
}
