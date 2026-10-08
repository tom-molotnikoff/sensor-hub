package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"example/sensorHub/email"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockEmailService struct{ mock.Mock }

func (m *mockEmailService) Settings(ctx context.Context) (gen.EmailSettings, error) {
	args := m.Called(ctx)
	return args.Get(0).(gen.EmailSettings), args.Error(1)
}

func (m *mockEmailService) Update(ctx context.Context, settings gen.EmailSettings) (gen.EmailSettings, error) {
	args := m.Called(ctx, settings)
	return args.Get(0).(gen.EmailSettings), args.Error(1)
}

func (m *mockEmailService) SendTest(ctx context.Context, recipient string) error {
	return m.Called(ctx, recipient).Error(0)
}

// emailRouter serves the email routes as a signed-in user with the given
// email address.
func emailRouter(service *mockEmailService, callerEmail string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1, Username: "admin", Email: callerEmail})
	})
	s := &Server{emailService: service}
	router.GET("/api/email/smtp", s.GetEmailSettings)
	router.PUT("/api/email/smtp", s.UpdateEmailSettings)
	router.POST("/api/email/smtp/test", s.SendTestEmail)
	return router
}

func serve(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestUpdateEmailSettings_RefusesInvalidSettingsWith400(t *testing.T) {
	service := &mockEmailService{}
	service.On("Update", mock.Anything, mock.Anything).
		Return(gen.EmailSettings{}, fmt.Errorf("%w: port must be from 1 to 65535, got 0", email.ErrInvalidSettings))

	w := serve(emailRouter(service, "admin@example.com"), http.MethodPut, "/api/email/smtp", map[string]any{"host": "smtp.example.com"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "port must be from 1 to 65535")
}

func TestSendTestEmail_SendsToTheCallersOwnAddress(t *testing.T) {
	service := &mockEmailService{}
	service.On("SendTest", mock.Anything, "admin@example.com").Return(nil)

	w := serve(emailRouter(service, "admin@example.com"), http.MethodPost, "/api/email/smtp/test", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	service.AssertExpectations(t)
}

func TestSendTestEmail_AnswersWithTheSMTPErrorAs502(t *testing.T) {
	service := &mockEmailService{}
	service.On("SendTest", mock.Anything, "admin@example.com").
		Return(&email.SendError{Err: errors.New("the SMTP server refused the login: 535 5.7.8 Authentication credentials invalid")})

	w := serve(emailRouter(service, "admin@example.com"), http.MethodPost, "/api/email/smtp/test", nil)

	require.Equal(t, http.StatusBadGateway, w.Code)
	var body gen.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "the SMTP server refused the login: 535 5.7.8 Authentication credentials invalid", body.Message)
}

func TestSendTestEmail_AnswersWith400WhenNothingCanBeSent(t *testing.T) {
	for name, err := range map[string]error{
		"no address":     email.ErrNoRecipient,
		"not configured": fmt.Errorf("%w: no SMTP password is set", email.ErrNotConfigured),
	} {
		t.Run(name, func(t *testing.T) {
			service := &mockEmailService{}
			service.On("SendTest", mock.Anything, "").Return(err)

			w := serve(emailRouter(service, ""), http.MethodPost, "/api/email/smtp/test", nil)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), err.Error())
		})
	}
}
