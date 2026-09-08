package api

import (
	"bytes"
	"encoding/json"
	"errors"
	db "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupRoleRouter() (*gin.Engine, *gin.RouterGroup, *Server, *MockRoleService) {
	mockService := new(MockRoleService)
	s := &Server{roleService: mockService}
	router := gin.New()
	apiGroup := router.Group("/api")
	return router, apiGroup, s, mockService
}

func TestListRoles(t *testing.T) {
	router, api, s, mockService := setupRoleRouter()
	api.GET("/roles", s.ListRoles)

	mockService.On("ListRoles", mock.Anything).Return([]db.RoleInfo{{Id: 1, Name: "admin"}}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/roles", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "admin")
}

func TestListPermissions(t *testing.T) {
	router, api, s, mockService := setupRoleRouter()
	api.GET("/permissions", s.ListPermissions)

	mockService.On("ListPermissions", mock.Anything).Return([]db.PermissionInfo{{Id: 1, Name: "read"}}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/permissions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "read")
}

func TestAssignPermission(t *testing.T) {
	router, _, s, mockService := setupRoleRouter()
	router.POST("/api/roles/:id/permissions", func(c *gin.Context) {
		var id int
		fmt.Sscan(c.Param("id"), &id)
		s.AssignPermission(c, id)
	})

	reqBody := gen.AssignPermissionJSONRequestBody{PermissionId: 10}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.On("AssignPermission", mock.Anything, 1, 10).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/roles/1/permissions", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRemovePermission(t *testing.T) {
	router, _, s, mockService := setupRoleRouter()
	router.DELETE("/api/roles/:id/permissions/:pid", func(c *gin.Context) {
		var id, pid int
		fmt.Sscan(c.Param("id"), &id)
		fmt.Sscan(c.Param("pid"), &pid)
		s.RemovePermission(c, id, pid)
	})

	mockService.On("RemovePermission", mock.Anything, 1, 10).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/roles/1/permissions/10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRolePermissions(t *testing.T) {
	router, _, s, mockService := setupRoleRouter()
	router.GET("/api/roles/:id/permissions", func(c *gin.Context) {
		var id int
		fmt.Sscan(c.Param("id"), &id)
		s.GetRolePermissions(c, id)
	})

	mockService.On("ListPermissionsForRole", mock.Anything, 1).Return([]db.PermissionInfo{{Id: 10, Name: "read"}}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/roles/1/permissions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "read")
}

func TestListRoles_ServiceError(t *testing.T) {
	router, api, s, mockService := setupRoleRouter()
	api.GET("/roles", s.ListRoles)

	mockService.On("ListRoles", mock.Anything).Return([]db.RoleInfo{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/roles", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListPermissions_ServiceError(t *testing.T) {
	router, api, s, mockService := setupRoleRouter()
	api.GET("/permissions", s.ListPermissions)

	mockService.On("ListPermissions", mock.Anything).Return([]db.PermissionInfo{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/permissions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetRolePermissions_InvalidID(t *testing.T) {
	router, _, s, _ := setupRoleRouter()
	router.GET("/api/roles/:id/permissions", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid role id"})
			return
		}
		s.GetRolePermissions(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/roles/invalid/permissions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRolePermissions_ServiceError(t *testing.T) {
	router, _, s, mockService := setupRoleRouter()
	router.GET("/api/roles/:id/permissions", func(c *gin.Context) {
		var id int
		fmt.Sscan(c.Param("id"), &id)
		s.GetRolePermissions(c, id)
	})

	mockService.On("ListPermissionsForRole", mock.Anything, 1).Return([]db.PermissionInfo{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/roles/1/permissions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAssignPermission_InvalidID(t *testing.T) {
	router, _, s, _ := setupRoleRouter()
	router.POST("/api/roles/:id/permissions", func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid role id"})
			return
		}
		s.AssignPermission(c, id)
	})

	reqBody := gen.AssignPermissionJSONRequestBody{PermissionId: 10}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/roles/invalid/permissions", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssignPermission_InvalidJSON(t *testing.T) {
	router, _, s, _ := setupRoleRouter()
	router.POST("/api/roles/:id/permissions", func(c *gin.Context) {
		var id int
		fmt.Sscan(c.Param("id"), &id)
		s.AssignPermission(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/roles/1/permissions", bytes.NewBufferString("invalid"))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssignPermission_ServiceError(t *testing.T) {
	router, _, s, mockService := setupRoleRouter()
	router.POST("/api/roles/:id/permissions", func(c *gin.Context) {
		var id int
		fmt.Sscan(c.Param("id"), &id)
		s.AssignPermission(c, id)
	})

	reqBody := gen.AssignPermissionJSONRequestBody{PermissionId: 10}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.On("AssignPermission", mock.Anything, 1, 10).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/roles/1/permissions", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRemovePermission_InvalidRoleID(t *testing.T) {
	router, _, s, _ := setupRoleRouter()
	router.DELETE("/api/roles/:id/permissions/:pid", func(c *gin.Context) {
		var id, pid int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid role id"})
			return
		}
		if _, err := fmt.Sscan(c.Param("pid"), &pid); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid permission id"})
			return
		}
		s.RemovePermission(c, id, pid)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/roles/invalid/permissions/10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemovePermission_InvalidPermissionID(t *testing.T) {
	router, _, s, _ := setupRoleRouter()
	router.DELETE("/api/roles/:id/permissions/:pid", func(c *gin.Context) {
		var id, pid int
		if _, err := fmt.Sscan(c.Param("id"), &id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid role id"})
			return
		}
		if _, err := fmt.Sscan(c.Param("pid"), &pid); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid permission id"})
			return
		}
		s.RemovePermission(c, id, pid)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/roles/1/permissions/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemovePermission_ServiceError(t *testing.T) {
	router, _, s, mockService := setupRoleRouter()
	router.DELETE("/api/roles/:id/permissions/:pid", func(c *gin.Context) {
		var id, pid int
		fmt.Sscan(c.Param("id"), &id)
		fmt.Sscan(c.Param("pid"), &pid)
		s.RemovePermission(c, id, pid)
	})

	mockService.On("RemovePermission", mock.Anything, 1, 10).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/roles/1/permissions/10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
