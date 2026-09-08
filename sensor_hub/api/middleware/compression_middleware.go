package middleware

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func Compression() gin.HandlerFunc {
	compress := gzip.Gzip(gzip.DefaultCompression)
	return func(ctx *gin.Context) {
		if websocket.IsWebSocketUpgrade(ctx.Request) {
			ctx.Next()
			return
		}
		compress(ctx)
	}
}
