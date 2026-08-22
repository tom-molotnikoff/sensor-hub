package middleware

import (
	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
)

var authService service.AuthServiceInterface
var apiKeyService service.ApiKeyServiceInterface

func InitAuthMiddleware(a service.AuthServiceInterface) {
	authService = a
}

func InitApiKeyMiddleware(a service.ApiKeyServiceInterface) {
	apiKeyService = a
}

func AuthRequired() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Check API key header first
		apiKey := ctx.GetHeader("X-API-Key")
		if apiKey != "" && apiKeyService != nil {
			user, err := apiKeyService.ValidateApiKey(ctx.Request.Context(), apiKey)
			if err != nil || user == nil {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			ctx.Set("currentUser", user)
			ctx.Set("authMethod", "api_key")
			ctx.Next()
			return
		}

		// Fall back to cookie auth
		cookieName := "sensor_hub_session"
		if appProps.AppConfig() != nil && appProps.AppConfig().AuthSessionCookieName != "" {
			cookieName = appProps.AppConfig().AuthSessionCookieName
		}
		token, err := ctx.Cookie(cookieName)
		if err != nil || token == "" {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		user, err := authService.ValidateSession(ctx.Request.Context(), token)
		if err != nil || user == nil {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if user.MustChangePassword {
			allowed := map[string]struct{}{
				"POST:/api/auth/login":    {},
				"POST:/api/auth/logout":   {},
				"GET:/api/auth/me":        {},
				"PUT:/api/users/password": {},
			}
			method := ctx.Request.Method
			cleanPath := path.Clean(ctx.Request.URL.Path)
			key := method + ":" + cleanPath
			if _, ok := allowed[key]; !ok {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		ctx.Set("currentUser", user)
		ctx.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		u, exists := ctx.Get("currentUser")
		if !exists {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		user := u.(*gen.User)
		if user == nil {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		isAdmin := false
		for _, r := range user.Roles {
			if r == "admin" {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			ctx.AbortWithStatus(http.StatusForbidden)
			return
		}
		ctx.Next()
	}
}
