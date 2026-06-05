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
	"strconv"
	"strings"

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

// ProxmoxClient é o cliente HTTP para a API REST do Proxmox VE.
type ProxmoxClient struct {
	baseURL     string
	node        string
	nodeHost    string // hostname sem porta, para conexões WebSocket diretas
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
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
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
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

// nextVMID obtém o próximo ID de VM disponível no cluster.
// Proxmox: GET /cluster/nextid
func (c *ProxmoxClient) nextVMID(ctx context.Context) (int, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/cluster/nextid", nil)
	if err != nil {
		return 0, fmt.Errorf("erro ao obter próximo VMID: %w", err)
	}
	// Proxmox pode retornar nextid como número ou string dependendo da versão
	raw, err := decodeBody[json.RawMessage](resp)
	if err != nil {
		return 0, err
	}
	var id int
	if err := json.Unmarshal(raw, &id); err == nil {
		return id, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("formato inesperado de nextid: %s", string(raw))
	}
	return strconv.Atoi(s)
}

// CreateVM cria uma VM QEMU no nó Proxmox.
// Proxmox: POST /nodes/{node}/qemu
// Se req.VMID == 0, obtém automaticamente o próximo ID disponível via /cluster/nextid.
// Retorna o VMID atribuído e o UPID da tarefa assíncrona de criação.
func (c *ProxmoxClient) CreateVM(ctx context.Context, req VMCreateRequest) (int, string, error) {
	vmid := req.VMID
	if vmid == 0 {
		var err error
		vmid, err = c.nextVMID(ctx)
		if err != nil {
			return 0, "", err
		}
	}

	storage := req.Storage
	if storage == "" {
		storage = "local-zfs"
	}
	network := req.Network
	if network == "" {
		// model= explícito é necessário no Proxmox 8 — sem ele gera "duplicate key"
		network = "model=virtio,bridge=vmbr0"
	}

	params := url.Values{}
	params.Set("vmid", strconv.Itoa(vmid))
	params.Set("name", req.Name)
	params.Set("cores", strconv.Itoa(req.CPUCores))
	params.Set("memory", strconv.Itoa(req.MemoryMB))
	params.Set("scsihw", "virtio-scsi-pci")
	// ZFS usa raw; dir/NFS usa qcow2; LVM não usa format
	diskParam := fmt.Sprintf("%s:%d", storage, req.DiskGB)
	if storage == "local-zfs" || req.Storage == "local-zfs" {
		diskParam += ",format=raw"
	}
	params.Set("scsi0", diskParam)
	params.Set("ostype", req.OSType)
	params.Set("net0", network)
	if req.Pool != "" {
		params.Set("pool", req.Pool)
	}
	if len(req.Tags) > 0 {
		// Proxmox usa ";" como separador de tags
		params.Set("tags", strings.Join(req.Tags, ";"))
	}
	if req.ISO != "" {
		params.Set("ide2", req.ISO+",media=cdrom")
		params.Set("boot", "order=ide2;scsi0")
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/nodes/"+c.node+"/qemu", strings.NewReader(params.Encode()))
	if err != nil {
		return 0, "", fmt.Errorf("erro ao criar VM: %w", err)
	}
	taskID, err := decodeBody[string](resp)
	if err != nil {
		return 0, "", err
	}
	return vmid, taskID, nil
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

// DeleteVM destroi uma VM e seus discos.
// Proxmox: DELETE /nodes/{node}/qemu/{vmid}
func (c *ProxmoxClient) DeleteVM(ctx context.Context, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("erro ao deletar VM %d: %w", vmid, err)
	}
	resp.Body.Close()
	return nil
}

// GetVNCProxy cria um ticket VNC para acesso via terminal web (xterm.js + noVNC).
// Proxmox: POST /nodes/{node}/qemu/{vmid}/vncproxy  body: websocket=1
func (c *ProxmoxClient) GetVNCProxy(ctx context.Context, vmid int) (VNCProxyResponse, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", c.node, vmid)
	params := url.Values{}
	params.Set("websocket", "1")
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(params.Encode()))
	if err != nil {
		return VNCProxyResponse{}, fmt.Errorf("erro ao obter VNC proxy para VM %d: %w", vmid, err)
	}
	raw, err := decodeBody[pveVNCProxy](resp)
	if err != nil {
		return VNCProxyResponse{}, err
	}
	return VNCProxyResponse{Ticket: raw.Ticket, Port: raw.Port, Cert: raw.Cert}, nil
}

// ListVMs lista todas as VMs no nó configurado no cliente.
// Proxmox: GET /nodes/{node}/qemu
func (c *ProxmoxClient) ListVMs(ctx context.Context) ([]VMStatus, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", c.node)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar VMs: %w", err)
	}
	entries, err := decodeBody[[]pveQEMUEntry](resp)
	if err != nil {
		return nil, err
	}
	statuses := make([]VMStatus, len(entries))
	for i, e := range entries {
		statuses[i] = VMStatus{VMID: e.VMID, Name: e.Name, Status: e.Status}
	}
	return statuses, nil
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
	return TermProxyResponse{Ticket: raw.Ticket, Port: int(raw.Port), UPID: raw.UPID}, nil
}

// GetTaskStatus consulta o estado de uma tarefa assíncrona pelo UPID.
// Proxmox: GET /nodes/{node}/tasks/{upid}/status
func (c *ProxmoxClient) GetTaskStatus(ctx context.Context, upid string) (TaskStatus, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", c.node, url.PathEscape(upid))
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return TaskStatus{}, fmt.Errorf("erro ao obter status da tarefa: %w", err)
	}
	raw, err := decodeBody[pveTaskStatus](resp)
	if err != nil {
		return TaskStatus{}, err
	}
	return TaskStatus{UPID: raw.UPID, Status: raw.Status, ExitStatus: raw.ExitStatus, Type: raw.Type}, nil
}
