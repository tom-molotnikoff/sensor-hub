package middleware

import (
	"net/http"
	"strings"

	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
)

func RequirePermission(permission string) gin.HandlerFunc {
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

		for _, p := range user.Permissions {
			if strings.EqualFold(p, permission) {
				ctx.Next()
				return
			}
		}
		ctx.AbortWithStatus(http.StatusForbidden)
	}
}
