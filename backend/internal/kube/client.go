package kube

import (
	"errors"
	"fmt"
	"os"

	"github.com/proxlab/backend/internal/domain/repository"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const fieldManager = "proxlab"

// Runtime implementa repository.ContainerRuntime com client-go.
type Runtime struct {
	cs         kubernetes.Interface
	restConfig *rest.Config // nil nos testes; exigido por Exec
}

// A asserção de interface entra quando Exec/Logs existirem (Task 8).

// New carrega o kubeconfig. Arquivo ausente → repository.ErrRuntimeUnavailable
// (o servidor sobe mesmo assim; rotas de containers respondem 503).
func New(kubeconfigPath string) (*Runtime, error) {
	if _, err := os.Stat(kubeconfigPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, repository.ErrRuntimeUnavailable
		}
		return nil, fmt.Errorf("erro ao acessar kubeconfig: %w", err)
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig inválido: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente kubernetes: %w", err)
	}
	return &Runtime{cs: cs, restConfig: cfg}, nil
}

// NewWithClientset é usado em testes com fake.NewSimpleClientset.
func NewWithClientset(cs kubernetes.Interface) *Runtime {
	return &Runtime{cs: cs}
}
