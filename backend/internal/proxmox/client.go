package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/proxlab/backend/internal/config"
)

// parseHostname extrai apenas o hostname de uma URL completa (ex: "https://192.168.1.1:8006" → "192.168.1.1").
func parseHostname(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Hostname()
}

// ProxmoxClient é o cliente HTTP para operações de runtime na API REST do Proxmox VE
// (status, start, stop, termproxy). Criação e destruição de VMs ficam em internal/terraform.
type ProxmoxClient struct {
	baseURL     string
	node        string
	nodeHost    string // hostname sem porta, para conexões WebSocket diretas
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
	// pollInterval é o intervalo de WaitTask; zero → 1 s. Campo à parte para os testes.
	pollInterval time.Duration
}

// WsBase retorna a base URL do Proxmox com esquema WebSocket (wss://).
// Usado pelo TerminalHandler para montar a URL do vncwebsocket.
func (c *ProxmoxClient) WsBase() string {
	return strings.Replace(c.baseURL, "https://", "wss://", 1)
}

// Node retorna o nome do nó configurado (ex: "pve01").
func (c *ProxmoxClient) Node() string {
	return c.node
}

// AuthHeader retorna o valor do header Authorization para o Proxmox API.
func (c *ProxmoxClient) AuthHeader() string {
	return fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret)
}

// NewProxmoxClient cria um ProxmoxClient configurado a partir do Config da aplicação.
// TLS verification é desabilitada pois o ambiente de laboratório usa certificados self-signed.
func NewProxmoxClient(cfg *config.Config) *ProxmoxClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // rede interna da faculdade
	}
	return &ProxmoxClient{
		baseURL:     cfg.ProxmoxHost,
		node:        cfg.ProxmoxNode,
		nodeHost:    parseHostname(cfg.ProxmoxHost),
		tokenID:     cfg.ProxmoxTokenID,
		tokenSecret: cfg.ProxmoxTokenSecret,
		httpClient:  &http.Client{Transport: transport},
	}
}

// doRequest executa uma requisição HTTP autenticada contra a API REST do Proxmox.
// path é relativo a /api2/json, ex: "/nodes/pve/qemu".
// body deve ser um payload form-encoded (url.Values.Encode()) ou nil para GET/DELETE.
func (c *ProxmoxClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	fullURL := c.baseURL + "/api2/json" + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Authorization", c.AuthHeader())
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição ao Proxmox: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxmox retornou %d: %s", resp.StatusCode, string(raw))
	}
	return resp, nil
}

// decodeBody parseia o envelope {"data": T} de uma resposta do Proxmox.
func decodeBody[T any](resp *http.Response) (T, error) {
	defer resp.Body.Close()
	var envelope pveEnvelope[T]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		var zero T
		return zero, fmt.Errorf("erro ao decodificar resposta do Proxmox: %w", err)
	}
	return envelope.Data, nil
}

// GetVMStatus retorna o estado atual de runtime de uma VM.
// Proxmox: GET /nodes/{node}/qemu/{vmid}/status/current
func (c *ProxmoxClient) GetVMStatus(ctx context.Context, vmid int) (VMStatus, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return VMStatus{}, fmt.Errorf("erro ao obter status da VM %d: %w", vmid, err)
	}
	raw, err := decodeBody[pveVMStatus](resp)
	if err != nil {
		return VMStatus{}, err
	}
	return VMStatus{
		VMID:    raw.VMID,
		Name:    raw.Name,
		Status:  raw.Status,
		CPU:     raw.CPU,
		MemUsed: raw.Mem,
		MemMax:  raw.MaxMem,
		Uptime:  raw.Uptime,
	}, nil
}

// StartVM inicia uma VM parada de forma assíncrona.
// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/start
// Retorna o UPID da tarefa de inicialização.
func (c *ProxmoxClient) StartVM(ctx context.Context, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(""))
	if err != nil {
		return "", fmt.Errorf("erro ao iniciar VM %d: %w", vmid, err)
	}
	return decodeBody[string](resp)
}

// StopVM para uma VM em execução (hard stop — sem graceful shutdown).
// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/stop
// Retorna o UPID da tarefa de parada.
func (c *ProxmoxClient) StopVM(ctx context.Context, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(""))
	if err != nil {
		return "", fmt.Errorf("erro ao parar VM %d: %w", vmid, err)
	}
	return decodeBody[string](resp)
}

// RebootVM reinicia uma VM em execução (shutdown ACPI seguido de start).
// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/reboot
// Retorna o UPID da tarefa.
func (c *ProxmoxClient) RebootVM(ctx context.Context, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/reboot", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(""))
	if err != nil {
		return "", fmt.Errorf("erro ao reiniciar VM %d: %w", vmid, err)
	}
	return decodeBody[string](resp)
}

// WaitTask espera a tarefa terminar e devolve o exitstatus ("OK" em sucesso).
// Proxmox: GET /nodes/{node}/tasks/{upid}/status, a cada pollInterval até status == "stopped".
// Respeita o ctx (o chamador define o teto).
func (c *ProxmoxClient) WaitTask(ctx context.Context, upid string) (string, error) {
	interval := c.pollInterval
	if interval <= 0 {
		interval = time.Second
	}
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", c.node, url.PathEscape(upid))
	for {
		resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
		if err != nil {
			return "", fmt.Errorf("erro ao consultar tarefa %s: %w", upid, err)
		}
		st, err := decodeBody[pveTaskStatus](resp)
		if err != nil {
			return "", err
		}
		if st.Status == "stopped" {
			return st.ExitStatus, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("tarefa %s não terminou: %w", upid, ctx.Err())
		case <-time.After(interval):
		}
	}
}

// GetTermProxy abre um terminal de texto para a VM via Proxmox termproxy.
// Proxmox: POST /nodes/{node}/qemu/{vmid}/termproxy
// Retorna ticket e porta para conexão WebSocket direta ao nó Proxmox.
func (c *ProxmoxClient) GetTermProxy(ctx context.Context, vmid int) (TermProxyResponse, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/termproxy", c.node, vmid)
	log.Printf("[proxmox] GetTermProxy → POST %s", path)
	resp, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return TermProxyResponse{}, fmt.Errorf("erro ao obter termproxy para VM %d: %w", vmid, err)
	}
	raw, err := decodeBody[pveTermProxy](resp)
	if err != nil {
		return TermProxyResponse{}, err
	}
	log.Printf("[proxmox] termproxy ticket prefix=%.10s len=%d", raw.Ticket, len(raw.Ticket))
	return TermProxyResponse{Ticket: raw.Ticket, Port: int(raw.Port), UPID: raw.UPID, User: raw.User}, nil
}
