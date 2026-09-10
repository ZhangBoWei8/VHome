//go:build wireinject
// +build wireinject

package app

import (
	"context"

	"github.com/google/wire"

	agent8v "vhome/8v/agent"
	"vhome/internal/auth"
	"vhome/internal/config"
	"vhome/internal/http/handler"
	"vhome/internal/http/middleware"
	"vhome/internal/repository"
	"vhome/internal/service"
)

// InitializeAPP is the wire injector. Its body is discarded: `wire` reads the
// signature and the provider list, works out the construction order from the
// parameter and return types, and writes wire_gen.go. Regenerate with
// `go generate ./internal/app` after changing any constructor signature.
func InitializeAPP(ctx context.Context, envFile config.EnvFile) (*APP, func(), error) {
	wire.Build(
		config.LoadConfig,
		// HTTPConfig is intentionally absent: nothing in the graph consumes
		// it, and wire rejects providers that would go unused.
		wire.FieldsOf(new(config.Config),
			"App", "DB", "Auth", "Security", "Storage"),

		provideDB,
		repository.ProviderSet,

		provideSessionTTL,
		provideEncryptionKey,
		auth.NewSecretBox,
		service.ProviderSet,

		agent8v.ProviderSet,

		middleware.ProviderSet,
		handler.ProviderSet,

		provideEngine,
		wire.Struct(new(Workers), "*"),
		newAPP,
	)

	return nil, nil, nil
}
