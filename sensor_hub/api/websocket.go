package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"example/sensorHub/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: checkOrigin}

// checkOrigin refuses a browser on another site opening a WebSocket with the
// user's cookie. It allows an upgrade whose Origin host is the request's own
// Host or the host of SENSOR_HUB_ALLOWED_ORIGIN, and one with no Origin at all,
// as non-browser clients send none.
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil || originURL.Host == "" {
		return false
	}
	if strings.EqualFold(originURL.Host, r.Host) {
		return true
	}
	allowedURL, err := url.Parse(os.Getenv("SENSOR_HUB_ALLOWED_ORIGIN"))
	return err == nil && strings.EqualFold(originURL.Host, allowedURL.Host)
}

func createPushWebSocket(ctx *gin.Context, topic string) *websocket.Conn {
	slog.Debug("WebSocket upgrade request", "topic", topic, "origin", ctx.GetHeader("Origin"), "remote", ctx.Request.RemoteAddr)

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		slog.Error("failed to set websocket upgrade", "error", err)
		return nil
	}
	slog.Debug("WebSocket connection established, registering to hub", "topic", topic)
	ws.Register(conn, []string{topic})
	return conn
}
