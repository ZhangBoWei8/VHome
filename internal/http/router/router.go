package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
	"vhome/internal/http/middleware"
)

// Register only mounts routes. Constructing handlers and guards is the
// injector's job, so adding a service never touches this signature.
func Register(engine *gin.Engine, h handler.Handlers, g middleware.Guards) {
	api := engine.Group("/api/v1")

	registerHealthRoutes(api, h.Health)

	registerBootstrapRoutes(api, h.Bootstrap)

	registerIdentityRoutes(api, h.Identity, g.Session)

	registerMemberAndPantryRoutes(api, h.Member, h.Pantry, h.Home, g.Session, g.CSRF)

	registerMealRoutes(api, h.Meal, g.Session, g.CSRF)

	registerExpenseRoutes(api, h.Expense, g.Session, g.CSRF)
	registerMemoRoutes(api, h.Memo, g.Session, g.CSRF)
	registerNotificationSettingsRoutes(api, h.Notification, g.Session, g.CSRF)
	registerAgentRoutes(api, h.Agent, g.Session, g.CSRF)
}
