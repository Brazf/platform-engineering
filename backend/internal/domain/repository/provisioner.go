package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrWorkspaceNotFound indica que não há workspace Terraform para a VM
// (ex.: VM criada antes da migração para Terraform).
var ErrWorkspaceNotFound = errors.New("workspace terraform não encontrado")

// ErrGPUNotPermitted indica que o token do Proxmox não tem Mapping.Use no resource mapping da GPU.
// A correção é ACL no Proxmox (fora do código).
var ErrGPUNotPermitted = errors.New("token sem permissão Mapping.Use no resource mapping da GPU")

// VMSpec descreve a VM a ser provisionada. Independente da ferramenta de provisionamento.
type VMSpec struct {
	Name       string
	CPUCores   int
	MemoryMB   int
	DiskGB     int
	Storage    string // storage Proxmox. Vazio → "local-zfs"
	OSType     string // "l26" | "win10" | "win11" | "other"
	ISO        string // ex: "local:iso/ubuntu-22.04.iso"; vazio = sem cdrom
	Bridge     string // bridge de rede. Vazio → "vmbr0"
	Pool       string // resource pool do Proxmox
	Tags       []string
	GPUMapping string // nome do resource mapping PCI; vazio = sem GPU

	// Modo clone: quando CloneVMID > 0 a VM é clonada de um template cloud-init
	// (herda disco de sistema, BIOS e agent) e liga ao criar. ISO e OSType são ignorados.
	CloneVMID         int
	CloudInitUser     string
	CloudInitPassword string
	CloudInitSSHKeys  []string // chaves públicas autorizadas para CloudInitUser
	StaticIP          string   // CIDR, ex: "10.0.0.50/24". Vazio → DHCP
	Gateway           string   // obrigatório quando StaticIP != ""
}

// Provisioner é o port para criação e destruição de VMs no Proxmox.
// Implementado por internal/terraform.Runner.
type Provisioner interface {
	// Provision cria a VM e retorna o VMID alocado pelo Proxmox.
	// workspaceID identifica o estado da VM (é o UUID da entidade VM).
	Provision(ctx context.Context, workspaceID uuid.UUID, spec VMSpec) (vmid int, err error)

	// Deprovision destrói a VM e remove o workspace.
	// Retorna ErrWorkspaceNotFound se o workspace não existir.
	Deprovision(ctx context.Context, workspaceID uuid.UUID) error
}
