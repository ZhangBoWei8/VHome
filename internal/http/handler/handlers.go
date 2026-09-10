package handler

// Handlers is the complete set of HTTP handlers the router mounts. Wire fills
// every field automatically, so adding a handler means adding one field here
// and one constructor to ProviderSet — nothing else changes.
type Handlers struct {
	Health       *HealthHandler
	Bootstrap    *BootstrapHandler
	Identity     *IdentityHandler
	Member       *MemberHandler
	Pantry       *PantryHandler
	Meal         *MealHandler
	Expense      *ExpenseHandler
	Memo         *MemoHandler
	Notification *NotificationHandler
	Home         *HomeHandler
	Agent        *AgentHandler
}
