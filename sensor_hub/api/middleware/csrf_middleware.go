package middleware

import (
	"net/http"

	appProps "example/sensorHub/application_properties"

	"github.com/gin-gonic/gin"
)

func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
			// Skip CSRF for API key auth (not vulnerable to CSRF)
			if c.GetHeader("X-API-Key") != "" {
				c.Next()
				return
			}
			path := c.Request.URL.Path
			if path == "/api/auth/login" || path == "/api/auth/logout" {
				c.Next()
				return
			}
			cookieName := "sensor_hub_session"
			if appProps.AppConfig() != nil && appProps.AppConfig().AuthSessionCookieName != "" {
				cookieName = appProps.AppConfig().AuthSessionCookieName
			}
			token, err := c.Cookie(cookieName)
			if err != nil || token == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			clientCSRF := c.GetHeader("X-CSRF-Token")
			if clientCSRF == "" {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			serverCSRF, err := authService.GetCSRFForToken(c.Request.Context(), token)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if serverCSRF == "" || clientCSRF != serverCSRF {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		c.Next()
	}
}
