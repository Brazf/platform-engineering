package kube

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
)

// Logs abre o stream de logs do pod mais recente em Running. tailLines=200.
func (r *Runtime) Logs(ctx context.Context, ns, name string, follow bool) (io.ReadCloser, error) {
	pod, err := r.findRunningPod(ctx, ns, name)
	if err != nil {
		return nil, err
	}
	tail := int64(200)
	rc, err := r.cs.CoreV1().Pods(ns).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: name, Follow: follow, TailLines: &tail,
	}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir logs: %w", err)
	}
	return rc, nil
}
