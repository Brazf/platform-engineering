package kube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/proxlab/backend/internal/domain/repository"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type execSession struct {
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	sizeCh  chan remotecommand.TerminalSize
	cancel  context.CancelFunc
	done    chan error
}

func (s *execSession) Stdin() io.Writer  { return s.stdinW }
func (s *execSession) Stdout() io.Reader { return s.stdoutR }
func (s *execSession) Resize(cols, rows uint16) error {
	select {
	case s.sizeCh <- remotecommand.TerminalSize{Width: cols, Height: rows}:
	default:
	}
	return nil
}
func (s *execSession) Close() error {
	s.cancel()
	_ = s.stdinW.Close()
	_ = s.stdoutR.Close()
	return nil
}

// Next implementa remotecommand.TerminalSizeQueue.
func (s *execSession) Next() *remotecommand.TerminalSize {
	sz, ok := <-s.sizeCh
	if !ok {
		return nil
	}
	return &sz
}

// Exec abre um shell interativo no pod. Sem /bin/sh → repository.ErrNoShell.
func (r *Runtime) Exec(ctx context.Context, ns, name string, cols, rows uint16) (repository.TerminalSession, error) {
	if r.restConfig == nil {
		return nil, repository.ErrRuntimeUnavailable
	}
	pod, err := r.findRunningPod(ctx, ns, name)
	if err != nil {
		return nil, err
	}
	req := r.cs.CoreV1().RESTClient().Post().Resource("pods").Namespace(ns).Name(pod.Name).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: name,
			Command:   []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"},
			Stdin:     true, Stdout: true, Stderr: false, TTY: true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(r.restConfig, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("erro ao criar executor: %w", err)
	}

	sctx, cancel := context.WithCancel(ctx)
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	s := &execSession{stdinW: stdinW, stdoutR: stdoutR, sizeCh: make(chan remotecommand.TerminalSize, 1), cancel: cancel, done: make(chan error, 1)}
	s.sizeCh <- remotecommand.TerminalSize{Width: cols, Height: rows}

	go func() {
		err := exec.StreamWithContext(sctx, remotecommand.StreamOptions{Stdin: stdinR, Stdout: stdoutW, Tty: true, TerminalSizeQueue: s})
		if err != nil && strings.Contains(err.Error(), "executable file not found") {
			err = repository.ErrNoShell
		}
		_ = stdoutW.CloseWithError(err)
		s.done <- err
	}()
	return s, nil
}

// IsNoShell ajuda o handler a traduzir o erro assíncrono do stream.
func IsNoShell(err error) bool { return errors.Is(err, repository.ErrNoShell) }
