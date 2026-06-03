package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: restringir origens em produção
		return true
	},
}

// TerminalHandler gerencia conexões WebSocket de terminal para VMs.
type TerminalHandler struct {
	proxmoxClient repository.ProxmoxClientInterface
}

func NewTerminalHandler(proxmoxClient repository.ProxmoxClientInterface) *TerminalHandler {
	return &TerminalHandler{proxmoxClient: proxmoxClient}
}

// HandleTerminal faz upgrade da conexão HTTP para WebSocket e faz proxy
// do terminal VNC/SPICE da VM via API do Proxmox.
//
// GET /api/vms/:id/terminal (requer JWT válido no header)
func (h *TerminalHandler) HandleTerminal(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Stub: fecha conexão imediatamente enquanto não implementado
	_ = conn.WriteMessage(websocket.TextMessage, []byte("terminal not implemented"))
}
