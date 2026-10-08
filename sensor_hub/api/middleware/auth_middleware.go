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

// mustChangePasswordAllowed are the only routes a user who must change their
// password can reach, whether they authenticate with a session or an API key.
var mustChangePasswordAllowed = map[string]struct{}{
	"POST:/api/auth/login":    {},
	"POST:/api/auth/logout":   {},
	"GET:/api/auth/me":        {},
	"PUT:/api/users/password": {},
}

func AuthRequired() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		user, authMethod := authenticate(ctx)
		if user == nil {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if user.MustChangePassword {
			key := ctx.Request.Method + ":" + path.Clean(ctx.Request.URL.Path)
			if _, ok := mustChangePasswordAllowed[key]; !ok {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		ctx.Set("currentUser", user)
		ctx.Set("authMethod", authMethod)
		ctx.Next()
	}
}

// authenticate resolves the request's user from the X-API-Key header, or from
// the session cookie when no key is sent. It returns nil when neither
// identifies a user who may make requests; the services refuse disabled users.
func authenticate(ctx *gin.Context) (*gen.User, string) {
	if apiKey := ctx.GetHeader("X-API-Key"); apiKey != "" && apiKeyService != nil {
		user, err := apiKeyService.ValidateApiKey(ctx.Request.Context(), apiKey)
		if err != nil {
			return nil, ""
		}
		return user, "api_key"
	}

	cookieName := "sensor_hub_session"
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthSessionCookieName != "" {
		cookieName = cfg.AuthSessionCookieName
	}
	token, err := ctx.Cookie(cookieName)
	if err != nil || token == "" {
		return nil, ""
	}
	user, err := authService.ValidateSession(ctx.Request.Context(), token)
	if err != nil {
		return nil, ""
	}
	return user, ""
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
