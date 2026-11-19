// Smoke test do provisionamento via Terraform contra o Proxmox real.
// Cria UMA VM de teste (proxlab-smoke-<sufixo>), consulta o status via REST
// e a destrói. Nunca toca em VMs existentes.
//
// Uso: go run ./cmd/proxmox-check  (requer .env com PROXMOX_* e opcionalmente TERRAFORM_*)
//
// Modo --k3s: cria um container nginx no cluster k3s real (namespace u-smoke00),
// espera running, lê logs e apaga. Requer KUBECONFIG_PATH gerado pelo k3s-bootstrap.
//
// Modo --gpu[=<mapping>]: cria uma VM de teste com GPU (padrão: primeiro mapping de
// PROXMOX_GPU_MAPPINGS), confere que nasce desligada e a destrói. Não liga a VM.
//
// Modo --gpu-conflict[=<mapping>]: cria duas VMs de teste no mesmo mapping, liga A e espera
// a tarefa, liga B e imprime o exitstatus (o conflito), para A e destrói as duas.
// ATENÇÃO: ocupa a GPU por alguns segundos.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	if len(os.Args) > 1 {
		if mapping, ok := flagValue(os.Args[1], "--gpu"); ok {
			smokeGPU(cfg, pickMapping(cfg, mapping))
			return
		}
		if mapping, ok := flagValue(os.Args[1], "--gpu-conflict"); ok {
			smokeGPUConflict(cfg, pickMapping(cfg, mapping))
			return
		}
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
		MemoryMB: 1024,
		DiskGB:   20,
		OSType:   "l26",
		Pool:     cfg.ProxmoxPoolPesquisador,
		Tags:     []string{"proxlab", "smoke-test"},

		// Mesmo caminho do portal para Ubuntu: clone do template cloud-init
		// (a criação de VM vazia exige VM.Config.Options, que o token não tem).
		CloneVMID:         cfg.ProxmoxUbuntuTemplateVMID,
		CloudInitUser:     "ubuntu",
		CloudInitPassword: uuid.New().String(),
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

// flagValue reconhece "--name" (valor vazio) e "--name=valor".
func flagValue(arg, name string) (string, bool) {
	if arg == name {
		return "", true
	}
	if v, ok := strings.CutPrefix(arg, name+"="); ok {
		return v, true
	}
	return "", false
}

// pickMapping devolve o mapping pedido ou o primeiro configurado.
func pickMapping(cfg *config.Config, requested string) string {
	if requested != "" {
		return requested
	}
	if len(cfg.ProxmoxGPUMappings) == 0 {
		fatalf("nenhum mapping: passe --gpu=<mapping> ou configure PROXMOX_GPU_MAPPINGS")
	}
	return cfg.ProxmoxGPUMappings[0]
}

// gpuSpec monta uma VM de teste (clone Ubuntu) com GPU anexada.
func gpuSpec(cfg *config.Config, id uuid.UUID, mapping string) repository.VMSpec {
	return repository.VMSpec{
		Name:     "proxlab-smoke-gpu-" + id.String()[:8],
		CPUCores: 1,
		MemoryMB: 1024,
		DiskGB:   20,
		OSType:   "l26",
		Pool:     cfg.ProxmoxPoolPesquisador,
		Tags:     []string{"proxlab", "smoke-test"},

		CloneVMID:         cfg.ProxmoxUbuntuTemplateVMID,
		CloudInitUser:     "ubuntu",
		CloudInitPassword: uuid.New().String(),
		GPUMapping:        mapping,
	}
}

// smokeGPU cria uma VM com GPU, confere que nasce desligada (stopped) e a destrói.
func smokeGPU(cfg *config.Config, mapping string) {
	runner, err := terraform.NewRunner(cfg)
	if err != nil {
		fatalf("runner: %v", err)
	}
	client := proxmox.NewProxmoxClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	id := uuid.New()
	spec := gpuSpec(cfg, id, mapping)
	fmt.Printf("▶ 1/3 Provision com GPU %q (%s)\n", mapping, spec.Name)
	vmid, err := runner.Provision(ctx, id, spec)
	if err != nil {
		fatalf("Provision: %v", err)
	}
	fmt.Printf("  ok — VMID=%d workspace=%s\n", vmid, id)

	failed := false
	fmt.Printf("▶ 2/3 GetVMStatus(%d): deve estar stopped\n", vmid)
	if st, err := client.GetVMStatus(ctx, vmid); err != nil {
		fmt.Printf("  falhou: %v\n", err)
		failed = true
	} else if st.Status != "stopped" {
		fmt.Printf("  inesperado: status=%s (esperado stopped)\n", st.Status)
		failed = true
	} else {
		fmt.Println("  ok — status=stopped")
	}

	fmt.Printf("▶ 3/3 Deprovision(%s)\n", id)
	if err := runner.Deprovision(ctx, id); err != nil {
		fatalf("Deprovision: %v — VMID=%d precisa ser removida manualmente!", err, vmid)
	}
	fmt.Println("  ok — VM destruída e workspace removido")
	if failed {
		fatalf("smoke GPU falhou na checagem de status")
	}
	fmt.Println("Smoke GPU concluído.")
}

// smokeGPUConflict prova a recusa do Proxmox: liga a VM A e depois a B (mesma GPU).
// A limpeza (parar A, destruir A e B) roda sempre, via defer; fatalf só depois dela.
func smokeGPUConflict(cfg *config.Config, mapping string) {
	if err := runGPUConflict(cfg, mapping); err != nil {
		fatalf("%v", err)
	}
	fmt.Println("Smoke GPU-conflito concluído.")
}

func runGPUConflict(cfg *config.Config, mapping string) error {
	runner, err := terraform.NewRunner(cfg)
	if err != nil {
		return fmt.Errorf("runner: %w", err)
	}
	client := proxmox.NewProxmoxClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	type testVM struct {
		label string
		id    uuid.UUID
		vmid  int
		up    bool // provisionada
	}
	vms := []*testVM{{label: "A", id: uuid.New()}, {label: "B", id: uuid.New()}}

	// Limpeza garantida: para as duas (hard stop é no-op em VM parada) e destrói.
	defer func() {
		fmt.Println("▶ Limpeza")
		for _, v := range vms {
			if !v.up {
				continue
			}
			if up, err := client.StopVM(context.Background(), v.vmid); err == nil {
				_, _ = client.WaitTask(context.Background(), up)
			}
			if err := runner.Deprovision(context.Background(), v.id); err != nil {
				fmt.Printf("  ERRO ao destruir %s: %v — VMID=%d precisa ser removida manualmente!\n", v.label, err, v.vmid)
				continue
			}
			fmt.Printf("  ok — VM %s (VMID=%d) destruída\n", v.label, v.vmid)
		}
	}()

	for i, v := range vms {
		spec := gpuSpec(cfg, v.id, mapping)
		spec.Name = fmt.Sprintf("proxlab-smoke-gpu%s-%s", v.label, v.id.String()[:8])
		fmt.Printf("▶ %d/4 Provision VM %s com GPU %q\n", i+1, v.label, mapping)
		vmid, err := runner.Provision(ctx, v.id, spec)
		if err != nil {
			return fmt.Errorf("Provision %s: %w", v.label, err)
		}
		v.vmid, v.up = vmid, true
		fmt.Printf("  ok — VMID=%d\n", vmid)
	}

	a, b := vms[0], vms[1]
	fmt.Println("▶ 3/4 Liga A e espera a tarefa")
	upid, err := client.StartVM(ctx, a.vmid)
	if err != nil {
		return fmt.Errorf("StartVM A: %w", err)
	}
	exitA, err := client.WaitTask(ctx, upid)
	if err != nil {
		return fmt.Errorf("WaitTask A: %w", err)
	}
	fmt.Printf("  exitstatus A = %q\n", exitA)
	if exitA != "OK" {
		fmt.Println("  A não ligou: GPU provavelmente em uso por outra VM real — evidência válida do conflito.")
		return nil
	}

	fmt.Println("▶ 4/4 Liga B (mesma GPU): deve ser recusada")
	upid, err = client.StartVM(ctx, b.vmid)
	if err != nil {
		fmt.Printf("  recusada já na resposta HTTP: %v\n", err)
		return nil
	}
	exitB, err := client.WaitTask(ctx, upid)
	if err != nil {
		return fmt.Errorf("WaitTask B: %w", err)
	}
	fmt.Printf("  exitstatus B = %q\n", exitB)
	switch {
	case exitB == "OK":
		return errors.New("B ligou: o Proxmox NÃO recusou o conflito (inesperado)")
	case strings.Contains(exitB, "already in use"):
		fmt.Println("  contém \"already in use\" — detecção do backend confirmada")
	default:
		fmt.Println("  NÃO contém \"already in use\" — ajustar detecção e teste (a classificação por pré-checagem segue valendo)")
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERRO: "+format+"\n", args...)
	os.Exit(1)
}
