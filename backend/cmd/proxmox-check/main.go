// Smoke test do provisionamento via Terraform contra o Proxmox real.
// Cria UMA VM de teste (proxlab-smoke-<sufixo>), consulta o status via REST
// e a destrói. Nunca toca em VMs existentes.
//
// Uso: go run ./cmd/proxmox-check  (requer .env com PROXMOX_* e opcionalmente TERRAFORM_*)
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/proxmox"
	"github.com/proxlab/backend/internal/terraform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("configuração: %v", err)
	}
	if os.Getenv("TERRAFORM_WORKDIR") == "" {
		cfg.TerraformWorkdir = filepath.Join(os.TempDir(), "proxlab-smoke")
	}

	fmt.Printf("Proxmox %s (nó %s) · terraform=%s · workdir=%s\n\n",
		cfg.ProxmoxHost, cfg.ProxmoxNode, cfg.TerraformBin, cfg.TerraformWorkdir)

	runner, err := terraform.NewRunner(cfg)
	if err != nil {
		fatalf("runner: %v", err)
	}
	client := proxmox.NewProxmoxClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	id := uuid.New()
	spec := repository.VMSpec{
		Name:     "proxlab-smoke-" + id.String()[:8],
		CPUCores: 1,
		MemoryMB: 512,
		DiskGB:   4,
		OSType:   "l26",
		Tags:     []string{"proxlab", "smoke-test"},
	}

	fmt.Printf("▶ 1/3 Provision (%s)\n", spec.Name)
	vmid, err := runner.Provision(ctx, id, spec)
	if err != nil {
		fatalf("Provision: %v", err)
	}
	fmt.Printf("  ok — VMID=%d workspace=%s\n\n", vmid, id)

	fmt.Printf("▶ 2/3 GetVMStatus(%d) via REST\n", vmid)
	status, err := client.GetVMStatus(ctx, vmid)
	if err != nil {
		fmt.Printf("  falhou: %v (seguindo para destroy)\n\n", err)
	} else {
		fmt.Printf("  ok — status=%s name=%s\n\n", status.Status, status.Name)
	}

	fmt.Printf("▶ 3/3 Deprovision(%s)\n", id)
	if err := runner.Deprovision(ctx, id); err != nil {
		fatalf("Deprovision: %v — VMID=%d precisa ser removida manualmente!", err, vmid)
	}
	fmt.Println("  ok — VM destruída e workspace removido")
	fmt.Println("\nSmoke test concluído.")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERRO: "+format+"\n", args...)
	os.Exit(1)
}
