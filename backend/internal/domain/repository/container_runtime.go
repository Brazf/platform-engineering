package repository

import (
	"context"
	"errors"
	"io"

	"github.com/proxlab/backend/internal/domain/entity"
)

var (
	// ErrRuntimeUnavailable: kubeconfig ausente ou cluster inalcançável.
	ErrRuntimeUnavailable = errors.New("cluster de containers indisponível")
	// ErrNoShell: exec falhou porque a imagem não tem /bin/sh.
	ErrNoShell = errors.New("esta imagem não possui shell")
)

const (
	PhasePending = "pending"
	PhaseRunning = "running"
	PhaseFailed  = "failed"
	PhaseUnknown = "unknown"
)

type ContainerStatus struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

// ResourceQuotaSpec alimenta o ResourceQuota do namespace do usuário.
type ResourceQuotaSpec struct {
	CPUMillis int
	MemoryMB  int
	Pods      int
	StorageGB int
}

type TerminalSession interface {
	Stdin() io.Writer
	Stdout() io.Reader
	Resize(cols, rows uint16) error
	Close() error
}

// ContainerRuntime é a porta para o cluster. Implementada por internal/kube.Runtime.
type ContainerRuntime interface {
	EnsureNamespace(ctx context.Context, ns string, q ResourceQuotaSpec) error
	// Apply cria/atualiza Deployment, Secret, Service, Ingress e PVC. Idempotente.
	Apply(ctx context.Context, c *entity.Container, ingressDomain string) error
	Delete(ctx context.Context, ns, name string) error
	Status(ctx context.Context, ns string, names []string) (map[string]ContainerStatus, error)
	Exec(ctx context.Context, ns, name string, cols, rows uint16) (TerminalSession, error)
	Logs(ctx context.Context, ns, name string, follow bool) (io.ReadCloser, error)
}
