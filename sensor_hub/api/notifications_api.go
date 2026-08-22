package api

import (
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/ws"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) ListNotifications(c *gin.Context, params gen.ListNotificationsParams) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
	}
	offset := 0
	if params.Offset != nil {
		offset = *params.Offset
	}
	includeDismissed := false
	if params.IncludeDismissed != nil && *params.IncludeDismissed == gen.True {
		includeDismissed = true
	}

	notifs, err := s.notificationService.GetNotificationsForUser(ctx, userID, limit, offset, includeDismissed)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to get notifications", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, notifs)
}

func (s *Server) GetUnreadCount(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	count, err := s.notificationService.GetUnreadCount(ctx, userID)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to get unread count", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"count": count})
}

func (s *Server) MarkAsRead(c *gin.Context, id int) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	err := s.notificationService.MarkAsRead(ctx, userID, id)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to mark as read", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"message": "marked as read"})
}

func (s *Server) DismissNotification(c *gin.Context, id int) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	err := s.notificationService.Dismiss(ctx, userID, id)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to dismiss notification", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"message": "dismissed"})
}

func (s *Server) BulkMarkAsRead(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	err := s.notificationService.BulkMarkAsRead(ctx, userID)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to mark all as read", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"message": "all marked as read"})
}

func (s *Server) BulkDismiss(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	err := s.notificationService.BulkDismiss(ctx, userID)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to dismiss all", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"message": "all dismissed"})
}

func (s *Server) GetChannelPreferences(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	prefs, err := s.notificationService.GetChannelPreferences(ctx, userID)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to get preferences", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, prefs)
}

func (s *Server) SetChannelPreference(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.MustGet("currentUser").(*gen.User).Id

	var req gen.SetChannelPreferenceJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid request body", "error": err.Error()})
		return
	}

	emailEnabled := false
	if req.EmailEnabled != nil {
		emailEnabled = *req.EmailEnabled
	}
	inappEnabled := false
	if req.InappEnabled != nil {
		inappEnabled = *req.InappEnabled
	}

	pref := notifications.ChannelPreference{
		UserID:       userID,
		Category:     notifications.NotificationCategory(req.Category),
		EmailEnabled: emailEnabled,
		InAppEnabled: inappEnabled,
	}

	err := s.notificationService.SetChannelPreference(ctx, userID, pref)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to set preference", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"message": "preference saved"})
}

func (s *Server) NotificationsWebSocket(ctx *gin.Context) {
	userID := ctx.MustGet("currentUser").(*gen.User).Id

	topic := ws.UserNotificationTopic(userID)
	createPushWebSocket(ctx, topic)
}
