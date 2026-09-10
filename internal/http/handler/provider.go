package handler

import "github.com/google/wire"

// ProviderSet builds every handler plus the aggregate Handlers struct.
var ProviderSet = wire.NewSet(
	NewHealthHandler,
	NewBootstrapHandler,
	NewIdentityHandler,
	NewMemberHandler,
	NewPantryHandler,
	NewMealHandler,
	NewExpenseHandler,
	NewMemoHandler,
	NewNotificationHandler,
	NewHomeHandler,
	NewAgentHandler,

	wire.Struct(new(Handlers), "*"),
)
