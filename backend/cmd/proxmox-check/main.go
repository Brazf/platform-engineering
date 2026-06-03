// Ferramenta de diagnóstico de conectividade com o Proxmox VE.
// Uso: go run ./cmd/proxmox-check  (requer .env com variáveis PROXMOX_*)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/proxmox"
)

func main() {
	_ = godotenv.Load()

	cfg := &config.Config{
		ProxmoxHost:        mustEnv("PROXMOX_HOST"),
		ProxmoxNode:        getEnv("PROXMOX_NODE", "pve"),
		ProxmoxTokenID:     mustEnv("PROXMOX_TOKEN_ID"),
		ProxmoxTokenSecret: mustEnv("PROXMOX_TOKEN_SECRET"),
	}

	fmt.Printf("🔌 Conectando a %s (nó: %s)...\n\n", cfg.ProxmoxHost, cfg.ProxmoxNode)

	client := proxmox.NewProxmoxClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// --- Teste 1: listar VMs ---
	fmt.Println("▶ Teste 1 — ListVMs")
	vms, err := client.ListVMs(ctx)
	if err != nil {
		fatalf("ListVMs: %v", err)
	}
	fmt.Printf("  ✅ %d VM(s) no nó\n", len(vms))
	for _, vm := range vms {
		fmt.Printf("     VMID=%-4d  status=%-8s  name=%s\n", vm.VMID, vm.Status, vm.Name)
	}
	fmt.Println()

	// --- Teste 2: GetVMStatus da primeira VM ---
	if len(vms) > 0 {
		first := vms[0]
		fmt.Printf("▶ Teste 2 — GetVMStatus(VMID=%d)\n", first.VMID)
		status, err := client.GetVMStatus(ctx, first.VMID)
		if err != nil {
			fmt.Printf("  ❌ %v\n\n", err)
		} else {
			fmt.Printf("  ✅ status=%s  cpu=%.2f%%  mem=%dMB/%dMB  uptime=%ds\n\n",
				status.Status, status.CPU*100,
				status.MemUsed/1024/1024, status.MemMax/1024/1024,
				status.Uptime)
		}
	}

	// --- Teste 3: CreateVM com VM mínima ---
	fmt.Println("▶ Teste 3 — CreateVM (VM mínima de teste)")
	createReq := proxmox.VMCreateRequest{
		VMID:     0, // auto via /cluster/nextid
		Name:     "proxlab-test-delete-me",
		CPUCores: 1,
		MemoryMB: 512,
		DiskGB:   10,
		Storage:  "local-lvm",
		OSType:   "l26",
		Network:  "virtio,bridge=vmbr0",
	}
	vmid, taskID, err := client.CreateVM(ctx, createReq)
	if err != nil {
		fmt.Printf("  ❌ CreateVM falhou: %v\n\n", err)
		fmt.Println("  ℹ️  Tentando com storage 'local'...")

		createReq.Storage = "local"
		vmid, taskID, err = client.CreateVM(ctx, createReq)
		if err != nil {
			fmt.Printf("  ❌ CreateVM com 'local' também falhou: %v\n\n", err)
			fmt.Println("Diagnóstico concluído (criação não funcionou).")
			os.Exit(1)
		}
	}
	fmt.Printf("  ✅ VM criada — VMID=%d  taskID=%s\n", vmid, taskID)

	// Aguarda a tarefa de criação completar (máx 20s)
	fmt.Println("  ⏳ Aguardando task de criação...")
	taskDone := waitTask(ctx, client, taskID, 20*time.Second)
	if !taskDone {
		fmt.Println("  ⚠️  Task ainda em andamento após 20s — verificando assim mesmo")
	}
	fmt.Println()

	// --- Teste 4: verificar VM criada aparece na listagem ---
	fmt.Printf("▶ Teste 4 — GetVMStatus(VMID=%d) após criação\n", vmid)
	status, err := client.GetVMStatus(ctx, vmid)
	if err != nil {
		fmt.Printf("  ❌ GetVMStatus falhou: %v\n\n", err)
	} else {
		fmt.Printf("  ✅ status=%s  name=%s\n\n", status.Status, status.Name)
	}

	// --- Teste 5: deletar a VM de teste ---
	fmt.Printf("▶ Teste 5 — DeleteVM(VMID=%d)\n", vmid)
	if err := client.DeleteVM(ctx, vmid); err != nil {
		fmt.Printf("  ❌ DeleteVM falhou: %v\n", err)
		fmt.Printf("  ⚠️  VM VMID=%d precisa ser removida manualmente!\n\n", vmid)
	} else {
		fmt.Printf("  ✅ VM %d deletada com sucesso\n\n", vmid)
	}

	fmt.Println("Diagnóstico concluído.")
}

// waitTask aguarda uma task assíncrona do Proxmox completar.
func waitTask(ctx context.Context, client *proxmox.ProxmoxClient, upid string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ts, err := client.GetTaskStatus(ctx, upid)
		if err != nil {
			return false
		}
		if ts.Status == "stopped" {
			if ts.ExitStatus == "OK" {
				fmt.Printf("  ✅ Task concluída com sucesso\n")
			} else {
				fmt.Printf("  ❌ Task falhou: %s\n", ts.ExitStatus)
			}
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERRO: "+format+"\n", args...)
	os.Exit(1)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fatalf("variável obrigatória ausente: %s", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
