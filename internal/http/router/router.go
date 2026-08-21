package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/config"
	"vhome/internal/http/handler"
	"vhome/internal/http/middleware"
	"vhome/internal/service"
)

func Register(
	engine *gin.Engine,
	identityService *service.IdentityService,
	pantryService *service.PantryService,
	mealService *service.MealService,
	expenseService *service.ExpenseService,
	memoService *service.MemoService,
	notificationService *service.NotificationService,
	calendarSyncService *service.CalendarSyncService,
	dashboardService *service.DashboardService,
	authConfig config.AuthConfig,
) {
	bootstrapHandler := handler.NewBootstrapHandler(identityService, authConfig.CookieName)

	identityHandler := handler.NewIdentityHandler(identityService, authConfig.CookieName, authConfig.CookieSecure)

	requireSession := middleware.RequireSession(identityService, authConfig.CookieName)
	requireCSRF := middleware.RequireCSRF(identityService)
	memberHandler := handler.NewMemberHandler(identityService)
	pantryHandler := handler.NewPantryHandler(pantryService, "data/uploads")
	mealHandler := handler.NewMealHandler(mealService, "data/uploads")
	expenseHandler := handler.NewExpenseHandler(expenseService)
	memoHandler := handler.NewMemoHandler(memoService, calendarSyncService)
	notificationHandler := handler.NewNotificationHandler(notificationService)
	homeHandler := handler.NewHomeHandler(identityService, dashboardService)

	api := engine.Group("/api/v1")

	registerBootstrapRoutes(api, bootstrapHandler)

	registerIdentityRoutes(api, identityHandler, requireSession)

	registerMemberAndPantryRoutes(api, memberHandler, pantryHandler, homeHandler, requireSession, requireCSRF)

	registerMealRoutes(api, mealHandler, requireSession, requireCSRF)

	registerExpenseRoutes(api, expenseHandler, requireSession, requireCSRF)
	registerMemoRoutes(api, memoHandler, requireSession, requireCSRF)
	registerNotificationSettingsRoutes(api, notificationHandler, requireSession, requireCSRF)
}
