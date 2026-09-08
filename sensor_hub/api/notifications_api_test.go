package api

import (
	"bytes"
	"encoding/json"
	"errors"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupNotifRouter() (*gin.Engine, *gin.RouterGroup, *Server, *MockNotificationService) {
	mockService := new(MockNotificationService)
	s := &Server{notificationService: mockService}
	router := gin.New()
	apiGroup := router.Group("/api")
	return router, apiGroup, s, mockService
}

func TestListNotifications_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	limit := 10
	offset := 5
	api.GET("/notifications", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		params := gen.ListNotificationsParams{Limit: &limit, Offset: &offset}
		s.ListNotifications(c, params)
	})

	mockService.On("GetNotificationsForUser", mock.Anything, 1, 10, 5, false).
		Return([]notifications.UserNotification{{Notification: &notifications.Notification{Message: "test notif"}}}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/notifications", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test notif")
}

func TestListNotifications_IncludeDismissed(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	includeDismissed := gen.True
	api.GET("/notifications", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		params := gen.ListNotificationsParams{IncludeDismissed: &includeDismissed}
		s.ListNotifications(c, params)
	})

	mockService.On("GetNotificationsForUser", mock.Anything, 1, 50, 0, true).
		Return([]notifications.UserNotification{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/notifications", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListNotifications_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.GET("/notifications", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.ListNotifications(c, gen.ListNotificationsParams{})
	})

	mockService.On("GetNotificationsForUser", mock.Anything, 1, 50, 0, false).
		Return([]notifications.UserNotification{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/notifications", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetUnreadCount_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.GET("/notifications/unread", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.GetUnreadCount(c)
	})

	mockService.On("GetUnreadCount", mock.Anything, 1).Return(3, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/notifications/unread", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "3")
}

func TestGetUnreadCount_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.GET("/notifications/unread", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.GetUnreadCount(c)
	})

	mockService.On("GetUnreadCount", mock.Anything, 1).Return(0, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/notifications/unread", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMarkAsRead_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/:id/read", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid notification id"})
			return
		}
		s.MarkAsRead(c, id)
	})

	mockService.On("MarkAsRead", mock.Anything, 1, 42).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/42/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMarkAsRead_InvalidID(t *testing.T) {
	router, api, s, _ := setupNotifRouter()
	api.POST("/notifications/:id/read", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid notification id"})
			return
		}
		s.MarkAsRead(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/invalid/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMarkAsRead_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/:id/read", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid notification id"})
			return
		}
		s.MarkAsRead(c, id)
	})

	mockService.On("MarkAsRead", mock.Anything, 1, 42).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/42/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDismissNotification_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/:id/dismiss", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid notification id"})
			return
		}
		s.DismissNotification(c, id)
	})

	mockService.On("Dismiss", mock.Anything, 1, 42).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/42/dismiss", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDismissNotification_InvalidID(t *testing.T) {
	router, api, s, _ := setupNotifRouter()
	api.POST("/notifications/:id/dismiss", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid notification id"})
			return
		}
		s.DismissNotification(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/invalid/dismiss", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDismissNotification_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/:id/dismiss", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid notification id"})
			return
		}
		s.DismissNotification(c, id)
	})

	mockService.On("Dismiss", mock.Anything, 1, 42).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/42/dismiss", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestBulkMarkAsRead_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/bulk/read", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.BulkMarkAsRead(c)
	})

	mockService.On("BulkMarkAsRead", mock.Anything, 1).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/bulk/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBulkMarkAsRead_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/bulk/read", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.BulkMarkAsRead(c)
	})

	mockService.On("BulkMarkAsRead", mock.Anything, 1).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/bulk/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestBulkDismiss_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/bulk/dismiss", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.BulkDismiss(c)
	})

	mockService.On("BulkDismiss", mock.Anything, 1).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/bulk/dismiss", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBulkDismiss_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/bulk/dismiss", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.BulkDismiss(c)
	})

	mockService.On("BulkDismiss", mock.Anything, 1).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/bulk/dismiss", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetChannelPreferences_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.GET("/notifications/preferences", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.GetChannelPreferences(c)
	})

	prefs := []notifications.ChannelPreference{{Category: "threshold_alert", EmailEnabled: true}}
	mockService.On("GetChannelPreferences", mock.Anything, 1).Return(prefs, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/notifications/preferences", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "threshold_alert")
}

func TestGetChannelPreferences_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.GET("/notifications/preferences", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.GetChannelPreferences(c)
	})

	mockService.On("GetChannelPreferences", mock.Anything, 1).Return([]notifications.ChannelPreference{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/notifications/preferences", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSetChannelPreference_Success(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/preferences", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.SetChannelPreference(c)
	})

	emailEnabled := true
	inappEnabled := false
	reqBody := gen.SetChannelPreferenceJSONRequestBody{
		Category:     "threshold_alert",
		EmailEnabled: &emailEnabled,
		InappEnabled: &inappEnabled,
	}
	jsonBody, _ := json.Marshal(reqBody)

	expectedPref := notifications.ChannelPreference{
		UserID:       1,
		Category:     "threshold_alert",
		EmailEnabled: true,
		InAppEnabled: false,
	}
	mockService.On("SetChannelPreference", mock.Anything, 1, expectedPref).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/preferences", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetChannelPreference_InvalidJSON(t *testing.T) {
	router, api, s, _ := setupNotifRouter()
	api.POST("/notifications/preferences", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.SetChannelPreference(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/preferences", bytes.NewBufferString("invalid"))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetChannelPreference_ServiceError(t *testing.T) {
	router, api, s, mockService := setupNotifRouter()
	api.POST("/notifications/preferences", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.SetChannelPreference(c)
	})

	reqBody := gen.SetChannelPreferenceJSONRequestBody{Category: "threshold_alert"}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.On("SetChannelPreference", mock.Anything, 1, mock.AnythingOfType("notifications.ChannelPreference")).
		Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notifications/preferences", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
