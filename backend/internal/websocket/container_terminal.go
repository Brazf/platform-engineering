package websocket

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/proxlab/backend/internal/usecase"
)

type ContainerTerminalHandler struct {
	uc      *usecase.ContainerUseCase
	runtime repository.ContainerRuntime
}

func NewContainerTerminalHandler(uc *usecase.ContainerUseCase, runtime repository.ContainerRuntime) *ContainerTerminalHandler {
	return &ContainerTerminalHandler{uc: uc, runtime: runtime}
}

// Handle — GET /api/containers/:id/terminal?token=
//
// Protocolo cliente↔backend: entrada = mensagem de texto; resize = texto JSON
// {"type":"resize","cols":N,"rows":N}; saída do pod = mensagem binária.
func (h *ContainerTerminalHandler) Handle(c *gin.Context) {
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

	var writeMu sync.Mutex
	writeText := func(s string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(s))
	}

	session, err := h.runtime.Exec(c.Request.Context(), ctr.Namespace, ctr.Name, 80, 24)
	if err != nil {
		if errors.Is(err, kube.ErrPodNotRunning) {
			writeText("\r\nO container ainda não está em execução.\r\n")
		} else {
			log.Printf("[container-terminal] exec %s/%s: %v", ctr.Namespace, ctr.Name, err)
			writeText("\r\nNão foi possível abrir o terminal.\r\n")
		}
		return
	}
	defer session.Close()

	errc := make(chan error, 2)

	// pod → cliente
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := session.Stdout().Read(buf)
			if n > 0 {
				writeMu.Lock()
				werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				writeMu.Unlock()
				if werr != nil {
					errc <- werr
					return
				}
			}
			if err != nil {
				if kube.IsNoShell(err) {
					writeText("\r\n" + repository.ErrNoShell.Error() + "\r\n")
				} else if !errors.Is(err, io.EOF) {
					log.Printf("[container-terminal] stream %s/%s encerrado: %v", ctr.Namespace, ctr.Name, err)
				}
				errc <- err
				return
			}
		}
	}()

	// cliente → pod
	go func() {
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if mt == websocket.TextMessage && len(msg) > 0 && msg[0] == '{' {
				var r resizeMessage
				if json.Unmarshal(msg, &r) == nil && r.Type == "resize" && r.Cols > 0 && r.Rows > 0 {
					_ = session.Resize(r.Cols, r.Rows)
					continue
				}
			}
			if _, err := session.Stdin().Write(msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-errc:
			return
		case <-ping.C:
			writeMu.Lock()
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}
