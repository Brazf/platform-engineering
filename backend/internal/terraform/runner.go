package terraform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/domain/repository"
)

const (
	operationTimeout = 5 * time.Minute
	pluginCacheDir   = ".plugin-cache"
)

// Runner implementa repository.Provisioner executando o binário terraform
// em um workspace por VM: <root>/<vm-uuid>/{main.tf, terraform.tfstate}.
type Runner struct {
	execPath string
	root     string
	node     string
	env      map[string]string // env completo passado ao processo terraform
}

var _ repository.Provisioner = (*Runner)(nil)

// NewRunner cria a raiz dos workspaces e o cache de providers.
func NewRunner(cfg *config.Config) (*Runner, error) {
	if err := os.MkdirAll(filepath.Join(cfg.TerraformWorkdir, pluginCacheDir), 0o750); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório de trabalho do terraform: %w", err)
	}

	env := inheritedEnv()
	env["TF_PLUGIN_CACHE_DIR"] = filepath.Join(cfg.TerraformWorkdir, pluginCacheDir)
	env["PROXMOX_VE_ENDPOINT"] = cfg.ProxmoxHost
	env["PROXMOX_VE_API_TOKEN"] = cfg.ProxmoxTokenID + "=" + cfg.ProxmoxTokenSecret
	env["PROXMOX_VE_INSECURE"] = "true"

	return &Runner{
		execPath: cfg.TerraformBin,
		root:     cfg.TerraformWorkdir,
		node:     cfg.ProxmoxNode,
		env:      env,
	}, nil
}

// inheritedEnv copia os.Environ sem as variáveis que o terraform-exec
// proíbe em SetEnv (TF_LOG, TF_INPUT, TF_VAR_*, etc.).
func inheritedEnv() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		env[k] = v
	}
	for _, k := range tfexec.ProhibitedEnv(env) {
		delete(env, k)
	}
	return env
}

func (r *Runner) workspacePath(id uuid.UUID) string {
	return filepath.Join(r.root, id.String())
}

func (r *Runner) newTF(dir string) (*tfexec.Terraform, error) {
	tf, err := tfexec.NewTerraform(dir, r.execPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao iniciar terraform: %w", err)
	}
	if err := tf.SetEnv(r.env); err != nil {
		return nil, fmt.Errorf("erro ao configurar ambiente do terraform: %w", err)
	}
	tf.SetStderr(logWriter{prefix: "[terraform] "})
	// Diagnóstico: TERRAFORM_LOG_PATH grava o log DEBUG (inclui as requisições
	// HTTP do provider) num arquivo. Não afeta a saída normal.
	if p := os.Getenv("TERRAFORM_LOG_PATH"); p != "" {
		if err := tf.SetLog("DEBUG"); err != nil {
			return nil, fmt.Errorf("erro ao configurar log do terraform: %w", err)
		}
		if err := tf.SetLogPath(p); err != nil {
			return nil, fmt.Errorf("erro ao configurar caminho do log do terraform: %w", err)
		}
	}
	return tf, nil
}

// Provision escreve main.tf, roda init+apply e lê o output vmid.
// Em falha de apply, tenta destroy e remove o workspace para não deixar lixo.
func (r *Runner) Provision(ctx context.Context, workspaceID uuid.UUID, spec repository.VMSpec) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	dir := r.workspacePath(workspaceID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, fmt.Errorf("erro ao criar workspace: %w", err)
	}

	hcl, err := Render(spec, r.node)
	if err != nil {
		_ = os.RemoveAll(dir)
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), hcl, 0o640); err != nil {
		_ = os.RemoveAll(dir)
		return 0, fmt.Errorf("erro ao escrever main.tf: %w", err)
	}

	tf, err := r.newTF(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return 0, err
	}

	if err := tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		_ = os.RemoveAll(dir)
		return 0, fmt.Errorf("terraform init falhou: %w", err)
	}

	if err := tf.Apply(ctx); err != nil {
		log.Printf("[terraform] apply falhou no workspace %s, executando destroy best-effort", workspaceID)
		if derr := tf.Destroy(ctx); derr != nil {
			log.Printf("[terraform] destroy após falha também falhou: %v", derr)
		}
		_ = os.RemoveAll(dir)
		return 0, fmt.Errorf("terraform apply falhou: %w", err)
	}

	outputs, err := tf.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("erro ao ler outputs do terraform: %w", err)
	}
	raw, ok := outputs["vmid"]
	if !ok {
		return 0, errors.New("output vmid ausente no terraform")
	}
	var vmid int
	if err := json.Unmarshal(raw.Value, &vmid); err != nil {
		return 0, fmt.Errorf("output vmid inválido: %w", err)
	}
	return vmid, nil
}

// applyError traduz a falha do apply. Sem Mapping.Use o admin precisa ajustar a ACL
// do Proxmox; o texto bruto fica só no log.
func applyError(err error) error {
	if strings.Contains(err.Error(), "Mapping.Use") {
		log.Printf("[terraform] token sem Mapping.Use no resource mapping da GPU — ajustar ACL no Proxmox (PVEMappingUser em /mapping/pci/<mapping>): %v", err)
		return fmt.Errorf("terraform apply falhou: %w: %w", repository.ErrGPUNotPermitted, err)
	}
	return fmt.Errorf("terraform apply falhou: %w", err)
}

// Deprovision roda destroy e remove o workspace.
func (r *Runner) Deprovision(ctx context.Context, workspaceID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	dir := r.workspacePath(workspaceID)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return repository.ErrWorkspaceNotFound
		}
		return fmt.Errorf("erro ao acessar workspace: %w", err)
	}

	tf, err := r.newTF(dir)
	if err != nil {
		return err
	}
	if err := tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		return fmt.Errorf("terraform init falhou: %w", err)
	}
	if err := tf.Destroy(ctx); err != nil {
		return fmt.Errorf("terraform destroy falhou: %w", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("erro ao remover workspace: %w", err)
	}
	return nil
}

// logWriter encaminha stderr do terraform para o log da aplicação.
type logWriter struct{ prefix string }

func (w logWriter) Write(p []byte) (int, error) {
	for line := range strings.SplitSeq(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			log.Print(w.prefix + line)
		}
	}
	return len(p), nil
}
