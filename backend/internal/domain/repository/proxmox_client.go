package repository

import (
	"context"

	"github.com/proxlab/backend/internal/proxmox"
)

// ProxmoxClientInterface é o port para operações na API do Proxmox VE.
// Definido na camada de domínio para desacoplar os use cases da implementação HTTP concreta.
// Implementado por internal/proxmox.ProxmoxClient via duck typing.
type ProxmoxClientInterface interface {
	// CreateVM cria uma VM QEMU no nó Proxmox.
	// Proxmox: POST /nodes/{node}/qemu
	// Retorna o VMID atribuído e o UPID da tarefa assíncrona de criação.
	CreateVM(ctx context.Context, req proxmox.VMCreateRequest) (vmid int, taskID string, err error)

	// GetVMStatus retorna o estado atual de runtime de uma VM.
	// Proxmox: GET /nodes/{node}/qemu/{vmid}/status/current
	GetVMStatus(ctx context.Context, vmid int) (proxmox.VMStatus, error)

	// StartVM inicia uma VM parada de forma assíncrona.
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/start
	// Retorna o UPID da tarefa.
	StartVM(ctx context.Context, vmid int) (taskID string, err error)

	// StopVM para uma VM em execução (hard stop imediato).
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/stop
	// Retorna o UPID da tarefa.
	StopVM(ctx context.Context, vmid int) (taskID string, err error)

	// DeleteVM destroi uma VM e seus discos.
	// Proxmox: DELETE /nodes/{node}/qemu/{vmid}
	DeleteVM(ctx context.Context, vmid int) error

	// GetVNCProxy cria um ticket VNC para acesso via terminal web (xterm.js + noVNC).
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/vncproxy  body: websocket=1
	GetVNCProxy(ctx context.Context, vmid int) (proxmox.VNCProxyResponse, error)

	// ListVMs lista todas as VMs no nó configurado no cliente.
	// Proxmox: GET /nodes/{node}/qemu
	ListVMs(ctx context.Context) ([]proxmox.VMStatus, error)

	// GetTermProxy abre um terminal de texto para a VM via Proxmox termproxy.
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/termproxy
	// Retorna ticket e porta para conexão WebSocket direta ao nó Proxmox.
	GetTermProxy(ctx context.Context, vmid int) (proxmox.TermProxyResponse, error)

	// GetTaskStatus consulta o estado de uma tarefa assíncrona pelo UPID.
	// Proxmox: GET /nodes/{node}/tasks/{upid}/status
	GetTaskStatus(ctx context.Context, upid string) (proxmox.TaskStatus, error)
}
