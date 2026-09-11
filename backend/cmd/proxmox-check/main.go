// Smoke test do provisionamento via Terraform contra o Proxmox real.
// Cria UMA VM de teste (proxlab-smoke-<sufixo>), consulta o status via REST
// e a destrói. Nunca toca em VMs existentes.
//
// Uso: go run ./cmd/proxmox-check  (requer .env com PROXMOX_* e opcionalmente TERRAFORM_*)
//
// Modo --k3s: cria um container nginx no cluster k3s real (namespace u-smoke00),
// espera running, lê logs e apaga. Requer KUBECONFIG_PATH gerado pelo k3s-bootstrap.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/proxlab/backend/internal/proxmox"
	"github.com/proxlab/backend/internal/terraform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("configuração: %v", err)
	}
	if len(os.Args) > 1 && os.Args[1] == "--k3s" {
		smokeK3s(cfg)
		return
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

// smokeK3s cria um container nginx do catálogo no cluster real, espera running,
// lê logs e apaga. Só toca em recursos criados aqui (namespace u-smoke00).
func smokeK3s(cfg *config.Config) {
	rt, err := kube.New(cfg.KubeconfigPath)
	if err != nil {
		fatalf("kube: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ns := "u-smoke00"
	img, _ := entity.CatalogByKey("nginx")
	c := &entity.Container{
		ID: uuid.New(), UserID: uuid.New(), Name: "smoke-" + uuid.New().String()[:6], Image: img.Image,
		CPUMillis: 250, MemoryMB: 128, Port: img.Port, Protocol: img.Protocol, NodePort: 32000,
		Env: entity.EnvMap{}, Namespace: ns,
	}
	fmt.Printf("▶ 1/4 EnsureNamespace(%s)\n", ns)
	if err := rt.EnsureNamespace(ctx, ns, repository.ResourceQuotaSpec{CPUMillis: 2000, MemoryMB: 4096, Pods: 5, StorageGB: 20}); err != nil {
		fatalf("EnsureNamespace: %v", err)
	}
	fmt.Printf("▶ 2/4 Apply(%s)\n", c.Name)
	if err := rt.Apply(ctx, c, cfg.IngressDomain); err != nil {
		fatalf("Apply: %v", err)
	}
	fmt.Println("▶ 3/4 Aguardando running (limpa o container ao falhar)")
	for {
		st, err := rt.Status(ctx, ns, []string{c.Name})
		if err != nil {
			_ = rt.Delete(context.Background(), ns, c.Name)
			fatalf("Status: %v", err)
		}
		fmt.Printf("  %s %s\n", st[c.Name].Phase, st[c.Name].Message)
		if st[c.Name].Phase == repository.PhaseRunning {
			break
		}
		if st[c.Name].Phase == repository.PhaseFailed {
			_ = rt.Delete(ctx, ns, c.Name)
			fatalf("container falhou")
		}
		time.Sleep(3 * time.Second)
	}
	rc, err := rt.Logs(ctx, ns, c.Name, false)
	if err != nil {
		fatalf("Logs: %v", err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	fmt.Printf("  logs (%d bytes) ok\n", len(b))
	fmt.Printf("▶ 4/4 Delete — testar antes: curl http://%s:32000\n", cfg.K3SNodeHost())
	if err := rt.Delete(ctx, ns, c.Name); err != nil {
		fatalf("Delete: %v", err)
	}
	fmt.Println("Smoke k3s concluído.")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERRO: "+format+"\n", args...)
	os.Exit(1)
}
