package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/proxlab/backend/internal/config"
)

// ProxmoxClient é o cliente HTTP para a API REST do Proxmox VE.
type ProxmoxClient struct {
	baseURL     string
	node        string
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
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
		storage = "local-lvm"
	}
	network := req.Network
	if network == "" {
		network = "virtio,bridge=vmbr0"
	}

	params := url.Values{}
	params.Set("vmid", strconv.Itoa(vmid))
	params.Set("name", req.Name)
	params.Set("cores", strconv.Itoa(req.CPUCores))
	params.Set("memory", strconv.Itoa(req.MemoryMB))
	params.Set("scsihw", "virtio-scsi-pci")
	params.Set("scsi0", fmt.Sprintf("%s:%d,format=qcow2", storage, req.DiskGB))
	params.Set("ostype", req.OSType)
	params.Set("net0", network)
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
