package k3s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// waitSSH tenta conectar até o timeout do ctx; retorna o cliente pronto.
func waitSSH(ctx context.Context, host, user string, privPEM []byte, logf func(string, ...any)) (*ssh.Client, error) {
	signer, err := ssh.ParsePrivateKey(privPEM)
	if err != nil {
		return nil, fmt.Errorf("chave privada inválida: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // VM recém-criada na rede interna
		Timeout:         10 * time.Second,
	}
	addr := net.JoinHostPort(host, "22")
	for {
		client, err := ssh.Dial("tcp", addr, cfg)
		if err == nil {
			return client, nil
		}
		logf("ssh %s ainda indisponível: %v", addr, err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout aguardando SSH em %s: %w", addr, ctx.Err())
		case <-time.After(10 * time.Second):
		}
	}
}

// run executa "sudo sh -s" lendo o script de stdin; devolve stdout.
func run(client *ssh.Client, stdin io.Reader, out io.Writer) ([]byte, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir sessão ssh: %w", err)
	}
	defer sess.Close()
	var stdout bytes.Buffer
	sess.Stdout = io.MultiWriter(&stdout, out)
	sess.Stderr = out
	sess.Stdin = stdin
	if err := sess.Run("sudo sh -s"); err != nil {
		return stdout.Bytes(), fmt.Errorf("script remoto falhou: %w", err)
	}
	return stdout.Bytes(), nil
}
