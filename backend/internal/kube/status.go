package kube

import (
	"context"
	"fmt"

	"github.com/proxlab/backend/internal/domain/repository"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status lista os pods do namespace uma vez e resume por app. Sem pod → pending.
func (r *Runtime) Status(ctx context.Context, ns string, names []string) (map[string]repository.ContainerStatus, error) {
	out := make(map[string]repository.ContainerStatus, len(names))
	for _, n := range names {
		out[n] = repository.ContainerStatus{Phase: repository.PhasePending, Message: "aguardando pod"}
	}
	pods, err := r.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/managed-by=proxlab"})
	if err != nil {
		return nil, fmt.Errorf("erro ao listar pods: %w", err)
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		app := p.Labels["app"]
		if _, wanted := out[app]; !wanted {
			continue
		}
		st := summarize(p)
		// Vários pods (rollout): running vence pending; failed vence tudo.
		if cur := out[app]; cur.Phase == repository.PhaseFailed || (cur.Phase == repository.PhaseRunning && st.Phase == repository.PhasePending) {
			continue
		}
		out[app] = st
	}
	return out, nil
}

func summarize(p *corev1.Pod) repository.ContainerStatus {
	for _, cs := range p.Status.ContainerStatuses {
		if w := cs.State.Waiting; w != nil {
			switch w.Reason {
			case "ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff", "CreateContainerConfigError", "InvalidImageName":
				return repository.ContainerStatus{Phase: repository.PhaseFailed, Message: w.Reason + ": " + w.Message}
			}
		}
	}
	switch p.Status.Phase {
	case corev1.PodRunning:
		return repository.ContainerStatus{Phase: repository.PhaseRunning}
	case corev1.PodPending:
		return repository.ContainerStatus{Phase: repository.PhasePending, Message: "iniciando"}
	case corev1.PodFailed:
		return repository.ContainerStatus{Phase: repository.PhaseFailed, Message: p.Status.Reason}
	case corev1.PodSucceeded:
		return repository.ContainerStatus{Phase: repository.PhaseFailed, Message: "processo principal terminou"}
	default:
		return repository.ContainerStatus{Phase: repository.PhaseUnknown}
	}
}
