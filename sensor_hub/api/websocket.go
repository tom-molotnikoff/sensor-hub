package api

import (
	"log/slog"
	"net/http"
	"time"

	"example/sensorHub/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func createPushWebSocket(ctx *gin.Context, topic string) *websocket.Conn {
	// log request info for debugging
	origin := ctx.GetHeader("Origin")
	remote := ctx.Request.RemoteAddr
	cookies := ctx.Request.Header.Get("Cookie")
	slog.Debug("WebSocket upgrade request", "topic", topic, "origin", origin, "remote", remote, "cookies", cookies)

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		slog.Error("failed to set websocket upgrade", "error", err)
		return nil
	}
	slog.Debug("WebSocket connection established, registering to hub", "topic", topic)
	ws.Register(conn, []string{topic})
	return conn
}

func createIntervalBasedWebSocket(ctx *gin.Context, topic string, methodToCall func() (any, error), intervalSeconds int) {
	// log request info for debugging
	origin := ctx.GetHeader("Origin")
	remote := ctx.Request.RemoteAddr
	cookies := ctx.Request.Header.Get("Cookie")
	slog.Debug("interval WebSocket upgrade request", "topic", topic, "origin", origin, "remote", remote, "cookies", cookies)

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		slog.Error("failed to set websocket upgrade", "error", err)
		return
	}

	ws.Register(conn, []string{topic})

	intervalDuration := time.Duration(intervalSeconds) * time.Second
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop()

	done := make(chan struct{})

	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				close(done)
				return
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			response, err := methodToCall()
			if err != nil {
				slog.Error("error fetching latest readings", "error", err)
				continue
			}
			if err := conn.WriteJSON(response); err != nil {
				slog.Warn("WebSocket closed or error writing interval message", "error", err)
				ws.Unregister(conn)
				return
			}
		case <-done:
			slog.Debug("WebSocket connection closed by client (interval)")
			ws.Unregister(conn)
			return
		}
	}
}
