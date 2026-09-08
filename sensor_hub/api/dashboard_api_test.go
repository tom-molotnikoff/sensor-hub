package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	gen "example/sensorHub/gen"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDashboardService struct {
	mock.Mock
}

func (m *mockDashboardService) ServiceListDashboards(ctx context.Context, userId int) ([]gen.Dashboard, error) {
	args := m.Called(ctx, userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.Dashboard), args.Error(1)
}

func (m *mockDashboardService) ServiceGetDashboard(ctx context.Context, id int) (*gen.Dashboard, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Dashboard), args.Error(1)
}

func (m *mockDashboardService) ServiceGetDefaultDashboard(ctx context.Context, userId int) (*gen.Dashboard, error) {
	args := m.Called(ctx, userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Dashboard), args.Error(1)
}

func (m *mockDashboardService) ServiceCreateDashboard(ctx context.Context, userId int, req gen.CreateDashboardRequest) (int, error) {
	args := m.Called(ctx, userId, req)
	return args.Int(0), args.Error(1)
}

func (m *mockDashboardService) ServiceUpdateDashboard(ctx context.Context, userId int, id int, req gen.UpdateDashboardRequest) error {
	args := m.Called(ctx, userId, id, req)
	return args.Error(0)
}

func (m *mockDashboardService) ServiceDeleteDashboard(ctx context.Context, userId int, id int) error {
	args := m.Called(ctx, userId, id)
	return args.Error(0)
}

func (m *mockDashboardService) ServiceShareDashboard(ctx context.Context, userId int, dashboardId int, targetUserId int) error {
	args := m.Called(ctx, userId, dashboardId, targetUserId)
	return args.Error(0)
}

func (m *mockDashboardService) ServiceSetDefaultDashboard(ctx context.Context, userId int, dashboardId int) error {
	args := m.Called(ctx, userId, dashboardId)
	return args.Error(0)
}

func setupDashboardRouter(method, path string, handler gin.HandlerFunc, userID int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.Handle(method, path, func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: userID, Username: "testuser"})
		handler(c)
	})
	return router
}

func sampleDashboard() gen.Dashboard {
	return gen.Dashboard{
		Id:        1,
		UserId:    1,
		Name:      "My Dashboard",
		Config:    `{"widgets":[],"breakpoints":{"lg":12,"md":10,"sm":6}}`,
		Shared:    false,
		IsDefault: false,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// --- List ---

func TestListDashboardsHandler(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	expected := []gen.Dashboard{sampleDashboard()}
	mockSvc.On("ServiceListDashboards", mock.Anything, 1).Return(expected, nil)

	router := setupDashboardRouter("GET", "/dashboards/", s.ListDashboards, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/dashboards/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "My Dashboard")
	mockSvc.AssertExpectations(t)
}

func TestListDashboardsHandler_EmptyReturnsArray(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceListDashboards", mock.Anything, 1).Return(nil, nil)

	router := setupDashboardRouter("GET", "/dashboards/", s.ListDashboards, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/dashboards/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", trimJSON(w.Body.Bytes()))
	mockSvc.AssertExpectations(t)
}

func TestListDashboardsHandler_Error(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceListDashboards", mock.Anything, 1).Return(nil, errors.New("db error"))

	router := setupDashboardRouter("GET", "/dashboards/", s.ListDashboards, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/dashboards/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- Get ---

func withDashboardID(s *Server, h func(*gin.Context, int)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid dashboard ID"})
			return
		}
		h(c, id)
	}
}

func TestGetDashboardHandler(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	d := sampleDashboard()
	mockSvc.On("ServiceGetDashboard", mock.Anything, 1).Return(&d, nil)

	router := setupDashboardRouter("GET", "/dashboards/:id", withDashboardID(s, s.GetDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/dashboards/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "My Dashboard")
	mockSvc.AssertExpectations(t)
}

func TestGetDashboardHandler_NotFound(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceGetDashboard", mock.Anything, 99).Return(nil, nil)

	router := setupDashboardRouter("GET", "/dashboards/:id", withDashboardID(s, s.GetDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/dashboards/99", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestGetDashboardHandler_InvalidID(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	router := setupDashboardRouter("GET", "/dashboards/:id", withDashboardID(s, s.GetDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/dashboards/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Create ---

func TestCreateDashboardHandler(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceCreateDashboard", mock.Anything, 1, mock.MatchedBy(func(req gen.CreateDashboardRequest) bool {
		return req.Name == "New Dashboard"
	})).Return(42, nil)

	body, _ := json.Marshal(gen.CreateDashboardRequest{Name: "New Dashboard"})
	router := setupDashboardRouter("POST", "/dashboards/", s.CreateDashboard, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/dashboards/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "42")
	mockSvc.AssertExpectations(t)
}

func TestCreateDashboardHandler_InvalidBody(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	router := setupDashboardRouter("POST", "/dashboards/", s.CreateDashboard, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/dashboards/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateDashboardHandler_Error(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceCreateDashboard", mock.Anything, 1, mock.Anything).Return(0, errors.New("db error"))

	body, _ := json.Marshal(gen.CreateDashboardRequest{Name: "Fail"})
	router := setupDashboardRouter("POST", "/dashboards/", s.CreateDashboard, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/dashboards/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- Update ---

func TestUpdateDashboardHandler(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceUpdateDashboard", mock.Anything, 1, 5, mock.Anything).Return(nil)

	body := []byte(`{"name":"Renamed"}`)
	router := setupDashboardRouter("PUT", "/dashboards/:id", withDashboardID(s, s.UpdateDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/dashboards/5", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dashboard updated")
	mockSvc.AssertExpectations(t)
}

func TestUpdateDashboardHandler_InvalidID(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	router := setupDashboardRouter("PUT", "/dashboards/:id", withDashboardID(s, s.UpdateDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/dashboards/xyz", bytes.NewReader([]byte(`{"name":"X"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateDashboardHandler_Error(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceUpdateDashboard", mock.Anything, 1, 5, mock.Anything).Return(errors.New("not owner"))

	body := []byte(`{"name":"X"}`)
	router := setupDashboardRouter("PUT", "/dashboards/:id", withDashboardID(s, s.UpdateDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/dashboards/5", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "not owner")
	mockSvc.AssertExpectations(t)
}

// --- Delete ---

func TestDeleteDashboardHandler(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceDeleteDashboard", mock.Anything, 1, 3).Return(nil)

	router := setupDashboardRouter("DELETE", "/dashboards/:id", withDashboardID(s, s.DeleteDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/dashboards/3", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dashboard deleted")
	mockSvc.AssertExpectations(t)
}

func TestDeleteDashboardHandler_InvalidID(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	router := setupDashboardRouter("DELETE", "/dashboards/:id", withDashboardID(s, s.DeleteDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/dashboards/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteDashboardHandler_Error(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceDeleteDashboard", mock.Anything, 1, 3).Return(errors.New("forbidden"))

	router := setupDashboardRouter("DELETE", "/dashboards/:id", withDashboardID(s, s.DeleteDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/dashboards/3", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- Share ---

func TestShareDashboardHandler(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceShareDashboard", mock.Anything, 1, 5, 2).Return(nil)

	body, _ := json.Marshal(gen.ShareDashboardRequest{TargetUserId: 2})
	router := setupDashboardRouter("POST", "/dashboards/:id/share", withDashboardID(s, s.ShareDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/dashboards/5/share", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dashboard shared")
	mockSvc.AssertExpectations(t)
}

func TestShareDashboardHandler_InvalidID(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	router := setupDashboardRouter("POST", "/dashboards/:id/share", withDashboardID(s, s.ShareDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/dashboards/abc/share", bytes.NewReader([]byte(`{"target_user_id":2}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestShareDashboardHandler_Error(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceShareDashboard", mock.Anything, 1, 5, 2).Return(errors.New("not found"))

	body, _ := json.Marshal(gen.ShareDashboardRequest{TargetUserId: 2})
	router := setupDashboardRouter("POST", "/dashboards/:id/share", withDashboardID(s, s.ShareDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/dashboards/5/share", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- Set Default ---

func TestSetDefaultDashboardHandler(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceSetDefaultDashboard", mock.Anything, 1, 7).Return(nil)

	router := setupDashboardRouter("PUT", "/dashboards/:id/default", withDashboardID(s, s.SetDefaultDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/dashboards/7/default", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Default dashboard set")
	mockSvc.AssertExpectations(t)
}

func TestSetDefaultDashboardHandler_InvalidID(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	router := setupDashboardRouter("PUT", "/dashboards/:id/default", withDashboardID(s, s.SetDefaultDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/dashboards/abc/default", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetDefaultDashboardHandler_Error(t *testing.T) {
	mockSvc := new(mockDashboardService)
	s := &Server{dashboardService: mockSvc}

	mockSvc.On("ServiceSetDefaultDashboard", mock.Anything, 1, 7).Return(errors.New("db error"))

	router := setupDashboardRouter("PUT", "/dashboards/:id/default", withDashboardID(s, s.SetDefaultDashboard), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/dashboards/7/default", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

func trimJSON(data []byte) string {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	b, _ := json.Marshal(v)
	return string(b)
}
