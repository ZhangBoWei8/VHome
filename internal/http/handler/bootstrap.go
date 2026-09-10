package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vhome/internal/config"
	"vhome/internal/http/response"
	"vhome/internal/service"
)

const (
	bootstrapRouteSetup     = "SETUP"
	bootstrapRouteLogin     = "LOGIN"
	bootstrapRouteDashboard = "DASHBOARD"
)

type BootstrapHandler struct {
	identityService *service.IdentityService
	sessionCookie   string
}

type BootstrapData struct {
	Initialized   bool   `json:"initialized"`
	Authenticated bool   `json:"authenticated"`
	NextRoute     string `json:"next_route"`
	APIVersion    string `json:"api_version"`
}

func NewBootstrapHandler(identityService *service.IdentityService, authConfig config.AuthConfig) *BootstrapHandler {
	return &BootstrapHandler{
		identityService: identityService,
		sessionCookie:   authConfig.CookieName,
	}
}

func (h *BootstrapHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()

	state, err := h.identityService.GetBootstrapState(ctx)
	if err != nil {
		_ = c.Error(err)

		response.WriteError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"服务暂时不可用",
		)

		return
	}

	if !state.Initialized {
		response.WriteData(
			c,
			http.StatusOK,
			BootstrapData{
				Initialized:   false,
				Authenticated: false,
				NextRoute:     bootstrapRouteSetup,
				APIVersion:    "v1",
			},
		)

		return
	}

	sessionToken, err := c.Cookie(h.sessionCookie)
	if err != nil || sessionToken == "" {
		response.WriteData(
			c,
			http.StatusOK,
			BootstrapData{
				Initialized:   true,
				Authenticated: false,
				NextRoute:     bootstrapRouteLogin,
				APIVersion:    "v1",
			},
		)

		return
	}

	_, err = h.identityService.AuthenticateSession(
		ctx,
		sessionToken,
	)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.WriteData(
				c,
				http.StatusOK,
				BootstrapData{
					Initialized:   true,
					Authenticated: false,
					NextRoute:     bootstrapRouteLogin,
					APIVersion:    "v1",
				},
			)

			return
		}

		_ = c.Error(err)

		response.WriteError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"服务暂时不可用",
		)

		return
	}

	response.WriteData(
		c,
		http.StatusOK,
		BootstrapData{
			Initialized:   true,
			Authenticated: true,
			NextRoute:     bootstrapRouteDashboard,
			APIVersion:    "v1",
		},
	)
}
