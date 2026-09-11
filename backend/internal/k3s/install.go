package k3s

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/repository"
)

//go:embed manifests/*.yaml
var manifests embed.FS

// WorkspaceID é o workspace Terraform fixo da VM do cluster.
var WorkspaceID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("proxlab-k3s"))

const sshUser = "ubuntu"

type InstallOptions struct {
	Provisioner    repository.Provisioner
	Workdir        string // $TERRAFORM_WORKDIR
	TemplateVMID   int
	NodeIP         string // CIDR
	Gateway        string
	CPU, MemoryMB  int
	DiskGB         int
	KubeconfigPath string
	Out            io.Writer
}

// Delimitadores de heredoc distintos (REGYAML, MANIFESTS) para não colidir com o YAML.
const installScript = `set -e
if ! command -v k3s >/dev/null 2>&1; then
  curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --write-kubeconfig-mode 644 --node-ip %[1]s" sh -
fi
until kubectl get nodes >/dev/null 2>&1; do sleep 3; done
mkdir -p /etc/rancher/k3s
cat > /etc/rancher/k3s/registries.yaml <<'REGYAML'
mirrors:
  "registry.proxlab.local":
    endpoint: ["http://registry.proxlab-system.svc.cluster.local:5000"]
REGYAML
cat <<'MANIFESTS' | kubectl apply -f -
%[2]s
MANIFESTS
kubectl -n proxlab-system rollout status deploy/registry --timeout=180s || true
`

// Install cria a VM (se ainda não existir workspace), instala k3s por SSH,
// aplica os manifests base e grava o kubeconfig. Idempotente.
func Install(ctx context.Context, o InstallOptions) error {
	logf := func(f string, a ...any) { fmt.Fprintf(o.Out, f+"\n", a...) }
	host, _, _ := strings.Cut(o.NodeIP, "/")
	if host == "" || o.Gateway == "" {
		return fmt.Errorf("K3S_NODE_IP e K3S_NODE_GATEWAY são obrigatórios")
	}

	keyDir := filepath.Join(o.Workdir, "k3s")
	privPEM, pub, err := ensureKeyPair(filepath.Join(keyDir, "id_ed25519"))
	if err != nil {
		return err
	}

	wsDir := filepath.Join(o.Workdir, WorkspaceID.String())
	if _, err := os.Stat(filepath.Join(wsDir, "terraform.tfstate")); err == nil {
		logf("▶ VM do cluster já provisionada (workspace %s); pulando terraform", WorkspaceID)
	} else {
		logf("▶ 1/4 Provisionando VM k3s-server (%s)", host)
		spec := repository.VMSpec{
			Name: "k3s-server", CPUCores: o.CPU, MemoryMB: o.MemoryMB, DiskGB: o.DiskGB,
			Tags:             []string{"proxlab", "k3s"},
			CloneVMID:        o.TemplateVMID,
			CloudInitUser:    sshUser,
			CloudInitSSHKeys: []string{strings.TrimSpace(string(pub))},
			StaticIP:         o.NodeIP,
			Gateway:          o.Gateway,
		}
		vmid, err := o.Provisioner.Provision(ctx, WorkspaceID, spec)
		if err != nil {
			return fmt.Errorf("provision: %w", err)
		}
		logf("  ok — VMID=%d", vmid)
	}

	logf("▶ 2/4 Aguardando SSH em %s", host)
	sshCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	client, err := waitSSH(sshCtx, host, sshUser, privPEM, logf)
	if err != nil {
		return err
	}
	defer client.Close()

	logf("▶ 3/4 Instalando k3s e manifests base")
	reg, err := manifests.ReadFile("manifests/registry.yaml")
	if err != nil {
		return fmt.Errorf("manifest embutido ausente: %w", err)
	}
	script := fmt.Sprintf(installScript, host, string(reg))
	if _, err := run(client, strings.NewReader(script), o.Out); err != nil {
		return err
	}

	logf("▶ 4/4 Copiando kubeconfig para %s", o.KubeconfigPath)
	kc, err := run(client, strings.NewReader("cat /etc/rancher/k3s/k3s.yaml\n"), io.Discard)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(o.KubeconfigPath), 0o750); err != nil {
		return fmt.Errorf("erro ao criar diretório do kubeconfig: %w", err)
	}
	if err := os.WriteFile(o.KubeconfigPath, rewriteServer(kc, host), 0o600); err != nil {
		return fmt.Errorf("erro ao gravar kubeconfig: %w", err)
	}
	logf("Cluster pronto. Reinicie o backend para carregar o kubeconfig.")
	return nil
}
