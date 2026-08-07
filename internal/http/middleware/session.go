package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/response"
	"vhome/internal/service"
)

const identityContextKey = "vhome.authenticated_identity"

func RequireSession(identityService *service.IdentityService, sessionCookie string) gin.HandlerFunc {
	return func(c *gin.Context) {
		plaintextToken, err :=
			c.Cookie(sessionCookie)
		if err != nil || plaintextToken == "" {
			response.WriteError(
				c,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"请先登录",
			)

			c.Abort()
			return
		}

		identity, err :=
			identityService.AuthenticateSession(
				c.Request.Context(),
				plaintextToken,
			)
		if err != nil {
			if errors.Is(
				err,
				service.ErrUnauthenticated,
			) {
				response.WriteError(
					c,
					http.StatusUnauthorized,
					"UNAUTHORIZED",
					"登录状态无效或已经过期",
				)
			} else {
				_ = c.Error(err)

				response.WriteError(
					c,
					http.StatusInternalServerError,
					"INTERNAL_ERROR",
					"服务暂时不可用",
				)
			}

			c.Abort()
			return
		}

		c.Set(identityContextKey, identity)
		c.Next()
	}
}

func CurrentIdentity(c *gin.Context) (service.AuthenticatedIdentity, bool) {
	value, exists := c.Get(identityContextKey)
	if !exists {
		return service.AuthenticatedIdentity{}, false
	}

	identity, ok :=
		value.(service.AuthenticatedIdentity)

	return identity, ok
}

func RequireCSRF(identityService *service.IdentityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := CurrentIdentity(c)
		if !ok || identityService.VerifyCSRFToken(identity, c.GetHeader("X-CSRF-Token")) != nil {
			response.WriteError(c, http.StatusForbidden, "INVALID_CSRF_TOKEN", "CSRF Token 无效")
			c.Abort()
			return
		}
		c.Next()
	}
}
