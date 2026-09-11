// k3s-bootstrap cria a VM do cluster k3s no Proxmox e instala o k3s via SSH.
// Executar uma vez: docker compose run --rm backend /app/k3s-bootstrap
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/k3s"
	"github.com/proxlab/backend/internal/terraform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("configuração: %v", err)
	}
	runner, err := terraform.NewRunner(cfg)
	if err != nil {
		fatalf("terraform: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = k3s.Install(ctx, k3s.InstallOptions{
		Provisioner: runner, Workdir: cfg.TerraformWorkdir, TemplateVMID: cfg.ProxmoxUbuntuTemplateVMID,
		NodeIP: cfg.K3SNodeIP, Gateway: cfg.K3SNodeGateway, Pool: cfg.K3SNodePool,
		CPU: cfg.K3SNodeCPU, MemoryMB: cfg.K3SNodeMemoryMB, DiskGB: cfg.K3SNodeDiskGB,
		KubeconfigPath: cfg.KubeconfigPath, Out: os.Stdout,
	})
	if err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERRO: "+format+"\n", args...)
	os.Exit(1)
}
