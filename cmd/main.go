package main

import (
	"flag"
	"log/slog"
	"os"

	"vhome/internal/bootstrap"
)

func main() {
	envFile := flag.String("env-file", "", "path to environment configuration file")
	flag.Parse()

	app, cleanup, err := bootstrap.New(*envFile)
	if err != nil {
		slog.Error("application initialization failed", "error", err)
		os.Exit(1)
	}

	defer cleanup()

	defer func() {
		if err := app.Close(); err != nil {
			slog.Error(
				"application close failed",
				"error",
				err,
			)
		}
	}()

	slog.Info("vhome is starting", "environment", app.Config.App.Env, "listen_addr", app.Config.HTTP.Addr)

	if err := app.Engine.Run(app.Config.HTTP.Addr); err != nil {
		slog.Error("application failed to start", "error", err)
		os.Exit(1)
	}
}
