package repository

import (
	"context"

	"github.com/proxlab/backend/internal/proxmox"
)

// ProxmoxClientInterface é o port para operações de runtime na API REST do Proxmox VE
// (status, start, stop, terminal). Criação e destruição ficam em Provisioner.
// Implementado por internal/proxmox.ProxmoxClient via duck typing.
type ProxmoxClientInterface interface {
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

	// RebootVM reinicia uma VM em execução (shutdown ACPI + start).
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/reboot
	// Retorna o UPID da tarefa.
	RebootVM(ctx context.Context, vmid int) (taskID string, err error)

	// WaitTask espera a tarefa (UPID) terminar e devolve o exitstatus ("OK" em sucesso).
	// Proxmox: GET /nodes/{node}/tasks/{upid}/status
	WaitTask(ctx context.Context, upid string) (exitStatus string, err error)

	// GetTermProxy abre um terminal de texto para a VM via Proxmox termproxy.
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/termproxy
	// Retorna ticket e porta para conexão WebSocket direta ao nó Proxmox.
	GetTermProxy(ctx context.Context, vmid int) (proxmox.TermProxyResponse, error)
}
