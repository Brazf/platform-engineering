package websocket

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/middleware"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// TerminalHandler gerencia conexões WebSocket de terminal para VMs.
type TerminalHandler struct {
	proxmoxClient repository.ProxmoxClientInterface
	vmRepo        repository.VMRepository
	wsBase        string // wss://host:8006
	node          string // ex: "pve01"
	authHeader    string // PVEAPIToken=id=secret
}

func NewTerminalHandler(
	proxmoxClient repository.ProxmoxClientInterface,
	vmRepo repository.VMRepository,
	wsBase, node, authHeader string,
) *TerminalHandler {
	return &TerminalHandler{
		proxmoxClient: proxmoxClient,
		vmRepo:        vmRepo,
		wsBase:        wsBase,
		node:          node,
		authHeader:    authHeader,
	}
}

// HandleTerminal faz upgrade para WebSocket, obtém credenciais do termproxy Proxmox
// e faz proxy bidirecional entre o frontend e o vncwebsocket do Proxmox.
//
// Fluxo:
//  1. POST /nodes/{node}/qemu/{vmid}/termproxy → { ticket, port }
//  2. Dial wss://proxmox:8006/api2/json/nodes/{node}/qemu/{vmid}/vncwebsocket?port={port}&vncticket={ticket}
//     com Authorization: PVEAPIToken=... e Subprotocols: binary
//  3. Proxy bidirecional direto — ticket já está nos query params, não enviar como mensagem
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

	// Upgrade antes de qualquer I/O de terminal
	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	// Passo 1: obtém ticket e porta via termproxy
	termProxy, err := h.proxmoxClient.GetTermProxy(c.Request.Context(), vm.ProxmoxVMID)
	if err != nil {
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(fmt.Sprintf("\r\nErro ao obter termproxy: %v\r\n", err)))
		return
	}

	// Passo 2: conecta ao vncwebsocket do Proxmox passando port e ticket como query params.
	// O ticket foi obtido via POST /termproxy e autentica esta conexão WS.
	proxmoxWSURL := fmt.Sprintf(
		"%s/api2/json/nodes/%s/qemu/%d/vncwebsocket?port=%d&vncticket=%s",
		h.wsBase, h.node, vm.ProxmoxVMID,
		termProxy.Port, url.QueryEscape(termProxy.Ticket),
	)

	reqHeader := http.Header{
		"Authorization": []string{h.authHeader},
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // rede interna da faculdade
		Subprotocols:    []string{"binary"},
	}

	safeURL := fmt.Sprintf(
		"%s/api2/json/nodes/%s/qemu/%d/vncwebsocket?port=%d&vncticket=***",
		h.wsBase, h.node, vm.ProxmoxVMID, termProxy.Port,
	)
	log.Printf("[terminal] dial proxmox → %s", safeURL)

	proxmoxConn, httpResp, err := dialer.Dial(proxmoxWSURL, reqHeader)
	if err != nil {
		msg := fmt.Sprintf("erro ao conectar ao Proxmox vncwebsocket: %v", err)
		if httpResp != nil {
			log.Printf("[terminal] dial falhou: status=%s url=%s err=%v", httpResp.Status, safeURL, err)
			for k, v := range httpResp.Header {
				log.Printf("[terminal]   header %s: %s", k, strings.Join(v, ", "))
			}
			msg = fmt.Sprintf("erro ao conectar ao Proxmox vncwebsocket (%s): %v", httpResp.Status, err)
		} else {
			log.Printf("[terminal] dial falhou (sem resposta HTTP): url=%s err=%v", safeURL, err)
		}
		_ = clientConn.WriteMessage(websocket.TextMessage, []byte("\r\n"+msg+"\r\n"))
		return
	}
	defer proxmoxConn.Close()

	log.Printf("[terminal] conexão estabelecida com vncwebsocket vmid=%d", vm.ProxmoxVMID)
	log.Printf("[terminal] ticket len=%d prefix=%.20s", len(termProxy.Ticket), termProxy.Ticket)

	// Passo 3: envia ticket como primeira mensagem (TextMessage) — exigido pelo --vncticket-endpoint.
	if err := proxmoxConn.WriteMessage(websocket.TextMessage, []byte(termProxy.Ticket+"\n")); err != nil {
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(fmt.Sprintf("\r\nErro ao enviar ticket: %v\r\n", err)))
		return
	}

	// Passo 4: aguarda confirmação "OK" do Proxmox.
	_, ackMsg, err := proxmoxConn.ReadMessage()
	if err != nil {
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(fmt.Sprintf("\r\nErro ao aguardar confirmação do Proxmox: %v\r\n", err)))
		return
	}
	if strings.TrimSpace(string(ackMsg)) != "OK" {
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(fmt.Sprintf("\r\nProxmox rejeitou ticket: %q\r\n", string(ackMsg))))
		return
	}
	log.Printf("[terminal] ticket aceito pelo Proxmox vmid=%d", vm.ProxmoxVMID)

	// Proxy bidirecional
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
