package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/handler"
	"github.com/proxlab/backend/internal/middleware"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
)

func newRouter(h *handler.ContainerHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, uuid.New().String())
		c.Set(middleware.ContextKeyRole, "aluno")
	})
	r.GET("/api/containers/catalog", h.Catalog)
	r.POST("/api/containers", h.Create)
	return r
}

func TestCatalog(t *testing.T) {
	h := handler.NewContainerHandler(usecase.NewContainerUseCase(nil, nil, "", ""))
	w := httptest.NewRecorder()
	newRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/containers/catalog", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"key":"postgres"`)
}

func TestCreate_BadPayload(t *testing.T) {
	h := handler.NewContainerHandler(usecase.NewContainerUseCase(nil, nil, "", ""))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/containers", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	newRouter(h).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreate_RuntimeUnavailableIs503(t *testing.T) {
	h := handler.NewContainerHandler(usecase.NewContainerUseCase(nil, nil, "", ""))
	w := httptest.NewRecorder()
	body := `{"name":"ok","image":"nginx:1.27","cpu_millis":250,"memory_mb":256,"port":80,"protocol":"http"}`
	req := httptest.NewRequest(http.MethodPost, "/api/containers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	newRouter(h).ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
