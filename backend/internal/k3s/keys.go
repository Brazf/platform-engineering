package k3s

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// ensureKeyPair devolve (privada PEM, pública authorized_keys); gera se não existir.
func ensureKeyPair(privPath string) ([]byte, []byte, error) {
	priv, err := os.ReadFile(privPath)
	if err == nil {
		signer, perr := ssh.ParsePrivateKey(priv)
		if perr != nil {
			return nil, nil, fmt.Errorf("chave existente inválida: %w", perr)
		}
		return priv, ssh.MarshalAuthorizedKey(signer.PublicKey()), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("erro ao ler chave: %w", err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao gerar chave: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(key, "proxlab-k3s")
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao serializar chave: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)
	if err := os.MkdirAll(filepath.Dir(privPath), 0o750); err != nil {
		return nil, nil, fmt.Errorf("erro ao criar diretório da chave: %w", err)
	}
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return nil, nil, fmt.Errorf("erro ao gravar chave: %w", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao converter chave pública: %w", err)
	}
	return privPEM, ssh.MarshalAuthorizedKey(sshPub), nil
}
