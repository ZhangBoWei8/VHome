package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type sessionCookieManager struct {
	sessionName string
	csrfName    string
	secure      bool
}

func newSessionCookieManager(
	sessionName string,
	secure bool,
) sessionCookieManager {
	return sessionCookieManager{
		sessionName: sessionName,
		csrfName:    sessionName + "_csrf",
		secure:      secure,
	}
}

func (m sessionCookieManager) Set(
	c *gin.Context,
	sessionToken string,
	csrfToken string,
	expiresAt time.Time,
) {
	maxAge := int(
		time.Until(expiresAt).Seconds(),
	)
	if maxAge < 1 {
		maxAge = 1
	}

	http.SetCookie(
		c.Writer,
		&http.Cookie{
			Name:     m.sessionName,
			Value:    sessionToken,
			Path:     "/",
			Expires:  expiresAt,
			MaxAge:   maxAge,
			HttpOnly: true,
			Secure:   m.secure,
			SameSite: http.SameSiteLaxMode,
		},
	)

	http.SetCookie(
		c.Writer,
		&http.Cookie{
			Name:     m.csrfName,
			Value:    csrfToken,
			Path:     "/",
			Expires:  expiresAt,
			MaxAge:   maxAge,
			HttpOnly: false,
			Secure:   m.secure,
			SameSite: http.SameSiteLaxMode,
		},
	)
}

func (m sessionCookieManager) Clear(
	c *gin.Context,
) {
	expiredAt := time.Unix(1, 0).UTC()

	http.SetCookie(
		c.Writer,
		&http.Cookie{
			Name:     m.sessionName,
			Value:    "",
			Path:     "/",
			Expires:  expiredAt,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   m.secure,
			SameSite: http.SameSiteLaxMode,
		},
	)

	http.SetCookie(
		c.Writer,
		&http.Cookie{
			Name:     m.csrfName,
			Value:    "",
			Path:     "/",
			Expires:  expiredAt,
			MaxAge:   -1,
			HttpOnly: false,
			Secure:   m.secure,
			SameSite: http.SameSiteLaxMode,
		},
	)
}
