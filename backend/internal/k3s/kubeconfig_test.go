package k3s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteServer(t *testing.T) {
	in := "apiVersion: v1\nclusters:\n- cluster:\n    server: https://127.0.0.1:6443\n  name: default\n"
	out := rewriteServer([]byte(in), "10.0.0.50")
	assert.Contains(t, string(out), "server: https://10.0.0.50:6443")
	assert.NotContains(t, string(out), "127.0.0.1")
}
