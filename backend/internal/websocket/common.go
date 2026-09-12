package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/middleware"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// userFromContext lê o userID injetado pelo middleware Auth (header ou ?token=).
func userFromContext(c *gin.Context) (uuid.UUID, bool) {
	raw, _ := c.Get(middleware.ContextKeyUserID)
	s, _ := raw.(string)
	id, err := uuid.Parse(s)
	return id, err == nil
}

// resizeMessage é o único JSON aceito do cliente; qualquer outro texto é entrada do teclado.
type resizeMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}
