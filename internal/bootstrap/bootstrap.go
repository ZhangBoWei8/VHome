package bootstrap

import (
	"context"
	"fmt"
	"vhome/internal/app"
)

func New(envFile string) (*app.APP, error) {
	app, err := app.New(context.Background(), envFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	return app, nil
}
