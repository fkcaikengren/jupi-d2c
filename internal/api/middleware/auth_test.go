package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", BearerAuth("secret"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func do(t *testing.T, set func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	if set != nil {
		set(req)
	}
	newAuthRouter().ServeHTTP(w, req)
	return w
}

func TestAuth_MissingHeader(t *testing.T) {
	w := do(t, nil)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "missing auth header")
}

func TestAuth_InvalidToken(t *testing.T) {
	w := do(t, func(r *http.Request) { r.Header.Set("Authorization", "Bearer wrong") })
	assert.Equal(t, 403, w.Code)
	assert.Contains(t, w.Body.String(), "invalid token")
}

func TestAuth_BearerOK(t *testing.T) {
	w := do(t, func(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") })
	assert.Equal(t, 200, w.Code)
}

func TestAuth_BearerlessOK(t *testing.T) {
	w := do(t, func(r *http.Request) { r.Header.Set("Authorization", "secret") })
	assert.Equal(t, 200, w.Code)
}

func TestAuth_PrivateTokenOK(t *testing.T) {
	w := do(t, func(r *http.Request) { r.Header.Set("PRIVATE-TOKEN", "secret") })
	assert.Equal(t, 200, w.Code)
}