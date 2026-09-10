package bootstrap

import (
	"context"
	"fmt"

	"vhome/internal/app"
	"vhome/internal/config"
)

func New(envFile string) (*app.APP, func(), error) {
	application, cleanup, err := app.New(
		context.Background(),
		config.EnvFile(envFile),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create app: %w", err)
	}

	return application, cleanup, nil
}
