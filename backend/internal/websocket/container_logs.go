package websocket

import (
	"bufio"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/proxlab/backend/internal/usecase"
)

type ContainerLogsHandler struct {
	uc      *usecase.ContainerUseCase
	runtime repository.ContainerRuntime
}

func NewContainerLogsHandler(uc *usecase.ContainerUseCase, runtime repository.ContainerRuntime) *ContainerLogsHandler {
	return &ContainerLogsHandler{uc: uc, runtime: runtime}
}

// Handle — GET /api/containers/:id/logs?token=  (somente saída, texto linha a linha)
func (h *ContainerLogsHandler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de container inválido"})
		return
	}
	userID, ok := userFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido"})
		return
	}
	ctr, err := h.uc.FindOwned(c.Request.Context(), userID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "container não encontrado"})
		return
	}
	if h.runtime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": repository.ErrRuntimeUnavailable.Error()})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx := c.Request.Context()
	// Descarta leituras do cliente só para detectar fechamento.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Enquanto o pod não está Running, informa o status a cada 2 s.
	for {
		rc, err := h.runtime.Logs(ctx, ctr.Namespace, ctr.Name, true)
		if err == nil {
			defer rc.Close()
			sc := bufio.NewScanner(rc)
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			for sc.Scan() {
				if werr := conn.WriteMessage(websocket.TextMessage, sc.Bytes()); werr != nil {
					return
				}
			}
			return
		}
		if !errors.Is(err, kube.ErrPodNotRunning) {
			log.Printf("[container-logs] %s/%s: %v", ctr.Namespace, ctr.Name, err)
			_ = conn.WriteMessage(websocket.TextMessage, []byte("[proxlab] não foi possível abrir os logs"))
			return
		}
		st, serr := h.runtime.Status(ctx, ctr.Namespace, []string{ctr.Name})
		msg := "[proxlab] aguardando o container iniciar..."
		if serr == nil {
			if s := st[ctr.Name]; s.Message != "" {
				msg = "[proxlab] " + s.Phase + ": " + s.Message
			}
			if st[ctr.Name].Phase == repository.PhaseFailed {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(msg))
				return
			}
		}
		if werr := conn.WriteMessage(websocket.TextMessage, []byte(msg)); werr != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
