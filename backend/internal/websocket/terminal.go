package websocket

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/middleware"
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
	vmRepo        repository.VMRepository
	nodeHost      string // hostname do nó Proxmox, para conexões WebSocket diretas
}

func NewTerminalHandler(proxmoxClient repository.ProxmoxClientInterface, vmRepo repository.VMRepository, nodeHost string) *TerminalHandler {
	return &TerminalHandler{
		proxmoxClient: proxmoxClient,
		vmRepo:        vmRepo,
		nodeHost:      nodeHost,
	}
}

// HandleTerminal faz upgrade para WebSocket, obtém credenciais de termproxy do Proxmox
// e faz proxy bidirecional entre o frontend e o terminal da VM.
//
// GET /api/vms/:id/terminal?token=<jwt>
func (h *TerminalHandler) HandleTerminal(c *gin.Context) {
	vmUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de VM inválido"})
		return
	}

	rawUserID, _ := c.Get(middleware.ContextKeyUserID)
	userIDStr, _ := rawUserID.(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido"})
		return
	}

	vm, err := h.vmRepo.FindByID(c.Request.Context(), vmUUID)
	if err != nil || vm == nil || vm.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "VM não encontrada"})
		return
	}

	// Faz upgrade da conexão antes de qualquer I/O de terminal
	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	// Obtém credenciais de termproxy do Proxmox
	termProxy, err := h.proxmoxClient.GetTermProxy(c.Request.Context(), vm.ProxmoxVMID)
	if err != nil {
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(fmt.Sprintf("\r\nErro ao conectar ao terminal: %v\r\n", err)))
		return
	}

	// Conecta ao WebSocket do termproxy diretamente no nó Proxmox.
	// O subprotocolo "binary" é obrigatório para o Proxmox termproxy — sem ele o handshake falha.
	proxmoxWSURL := fmt.Sprintf("wss://%s:%d/", h.nodeHost, termProxy.Port)
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // rede interna
		Subprotocols:    []string{"binary"},
	}
	proxmoxConn, _, err := dialer.Dial(proxmoxWSURL, nil)
	if err != nil {
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(fmt.Sprintf("\r\nErro ao conectar ao Proxmox: %v\r\n", err)))
		return
	}
	defer proxmoxConn.Close()

	// Autentica enviando o ticket como primeira mensagem
	if err := proxmoxConn.WriteMessage(websocket.TextMessage, []byte(termProxy.Ticket+"\n")); err != nil {
		_ = clientConn.WriteMessage(websocket.TextMessage, []byte("\r\nErro de autenticação com Proxmox\r\n"))
		return
	}

	// Proxy bidirecional: Proxmox → cliente e cliente → Proxmox
	errc := make(chan error, 2)

	go func() {
		for {
			mt, msg, err := proxmoxConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := clientConn.WriteMessage(mt, msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	go func() {
		for {
			mt, msg, err := clientConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := proxmoxConn.WriteMessage(mt, msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	<-errc
}
