package handler

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vhome/internal/config"
	"vhome/internal/http/middleware"
	"vhome/internal/http/response"
	"vhome/internal/service"
)

const maxUserAgentLength = 512

type IdentityHandler struct {
	identityService *service.IdentityService
	cookies         sessionCookieManager
}

func NewIdentityHandler(identityService *service.IdentityService, authConfig config.AuthConfig) *IdentityHandler {
	return &IdentityHandler{
		identityService: identityService,

		cookies: newSessionCookieManager(
			authConfig.CookieName,
			authConfig.CookieSecure,
		),
	}
}

func (h *IdentityHandler) Setup(c *gin.Context) {
	var request SetupRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}

	result, err := h.identityService.Setup(
		c.Request.Context(),
		service.SetupInput{
			HouseholdName: request.Household.Name,

			HouseholdPassword: request.Household.Password,

			HouseholdAvatar: request.Household.Avatar,

			OwnerName:     request.Owner.Name,
			OwnerPassword: request.Owner.Password,

			CreatedIP: requestClientIP(c),
			UserAgent: requestUserAgent(c),
		},
	)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	h.cookies.Set(
		c,
		result.SessionToken,
		result.CSRFToken,
		result.SessionExpiresAt,
	)

	response.WriteData(
		c,
		http.StatusCreated,
		newSetupSessionData(result),
	)
}

func (h *IdentityHandler) RegisterMember(c *gin.Context) {
	var request RegisterMemberRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}

	result, err :=
		h.identityService.RegisterMember(
			c.Request.Context(),
			service.RegisterMemberInput{
				Name:     request.Name,
				Password: request.Password,

				HouseholdPassword: request.HouseholdPassword,
			},
		)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	response.WriteData(
		c,
		http.StatusCreated,
		RegistrationData{
			Member: newMemberData(result.Member),
		},
	)
}

func (h *IdentityHandler) Login(c *gin.Context) {
	var request LoginRequest

	// 自动读取并解析json到go结构体中
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}

	result, err := h.identityService.Login(
		c.Request.Context(),
		service.LoginInput{
			Name:     request.Name,
			Password: request.Password,

			CreatedIP: requestClientIP(c),
			UserAgent: requestUserAgent(c),
		},
	)

	if err != nil {
		writeServiceError(c, err)
		return
	}

	h.cookies.Set(
		c,
		result.SessionToken,
		result.CSRFToken,
		result.SessionExpiresAt,
	)

	response.WriteData(
		c,
		http.StatusOK,
		newLoginSessionData(result),
	)
}

func (h *IdentityHandler) CurrentSession(c *gin.Context) {
	identity, exists := middleware.CurrentIdentity(c)
	if !exists {
		writeServiceError(
			c,
			service.ErrUnauthenticated,
		)
		return
	}

	response.WriteData(
		c,
		http.StatusOK,
		newCurrentSessionData(identity),
	)
}

func (h *IdentityHandler) Logout(c *gin.Context) {
	identity, exists := middleware.CurrentIdentity(c)
	if !exists {
		writeServiceError(
			c,
			service.ErrUnauthenticated,
		)
		return
	}

	csrfToken := c.GetHeader("X-CSRF-Token")

	if err := h.identityService.Logout(
		c.Request.Context(),
		identity,
		csrfToken,
	); err != nil {
		writeServiceError(c, err)
		return
	}

	h.cookies.Clear(c)
	c.Status(http.StatusNoContent)
}

func requestClientIP(c *gin.Context) net.IP {
	return net.ParseIP(c.ClientIP())
}

func requestUserAgent(c *gin.Context) string {
	value := strings.TrimSpace(
		c.Request.UserAgent(),
	)

	runes := []rune(value)
	if len(runes) > maxUserAgentLength {
		runes = runes[:maxUserAgentLength]
	}

	return string(runes)
}
