package service

import "github.com/google/wire"

// ProviderSet is every application service. Wire resolves the ordering between
// them from their constructor signatures, so DashboardService depending on
// three other services needs no manual sequencing here.
var ProviderSet = wire.NewSet(
	NewIdentityService,
	NewPantryService,
	NewMealService,
	NewExpenseService,
	NewMemoService,
	NewNotificationService,
	NewCalendarSyncService,
	NewDashboardService,
	NewAgentMemoryService,
	NewAgentConversationService,
)
