package k3s

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestEnsureKeyPair_GeneratesOnceAndReuses(t *testing.T) {
	dir := t.TempDir()
	priv1, pub1, err := ensureKeyPair(filepath.Join(dir, "id_ed25519"))
	require.NoError(t, err)
	assert.True(t, len(pub1) > 11 && string(pub1[:11]) == "ssh-ed25519")
	_, err = ssh.ParsePrivateKey(priv1)
	require.NoError(t, err)

	priv2, pub2, err := ensureKeyPair(filepath.Join(dir, "id_ed25519"))
	require.NoError(t, err)
	assert.Equal(t, priv1, priv2)
	assert.Equal(t, pub1, pub2)
}
