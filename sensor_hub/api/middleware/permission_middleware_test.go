package middleware

import (
	gen "example/sensorHub/gen"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequirePermission_Granted(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	user := &gen.User{Id: 1, Permissions: []string{"test_perm"}}
	c.Set("currentUser", user)

	RequirePermission("test_perm")(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_Forbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	user := &gen.User{Id: 1, Permissions: []string{"other_perm"}}
	c.Set("currentUser", user)

	RequirePermission("test_perm")(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_NoUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	RequirePermission("test_perm")(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequirePermission_NoPermissions(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("currentUser", &gen.User{Id: 1})

	RequirePermission("test_perm")(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
