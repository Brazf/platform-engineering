package kube

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var errPodNotRunning = errors.New("container ainda não está em execução")

// ErrPodNotRunning é exportado para o handler de logs esperar o pod subir.
var ErrPodNotRunning = errPodNotRunning

func (r *Runtime) findRunningPod(ctx context.Context, ns, name string) (*corev1.Pod, error) {
	pods, err := r.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app=" + name})
	if err != nil {
		return nil, fmt.Errorf("erro ao listar pods: %w", err)
	}
	var best *corev1.Pod
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodRunning {
			continue
		}
		if best == nil || p.CreationTimestamp.After(best.CreationTimestamp.Time) {
			best = p
		}
	}
	if best == nil {
		return nil, errPodNotRunning
	}
	return best, nil
}
