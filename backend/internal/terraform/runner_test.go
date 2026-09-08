package terraform_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTerraform escreve um script que responde a `version -json`, registra os
// argumentos em <dir>/calls.log e opcionalmente falha no apply.
func fakeTerraform(t *testing.T, dir string, failApply bool) string {
	t.Helper()
	exit := "0"
	if failApply {
		exit = "1"
	}
	script := `#!/bin/sh
echo "$@" >> "` + filepath.Join(dir, "calls.log") + `"
case "$1" in
  version) echo '{"terraform_version":"1.8.3","platform":"linux_amd64","provider_selections":{},"terraform_outdated":false}' ;;
  init) exit 0 ;;
  apply) exit ` + exit + ` ;;
  destroy) exit 0 ;;
  output) echo '{"vmid":{"sensitive":false,"type":"number","value":123}}' ;;
  *) exit 0 ;;
esac
`
	path := filepath.Join(dir, "terraform")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func newTestRunner(t *testing.T, failApply bool) (*terraform.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	bin := fakeTerraform(t, dir, failApply)
	cfg := &config.Config{
		ProxmoxHost:        "https://pve:8006",
		ProxmoxNode:        "pve01",
		ProxmoxTokenID:     "user@pam!tok",
		ProxmoxTokenSecret: "secret",
		TerraformBin:       bin,
		TerraformWorkdir:   filepath.Join(dir, "work"),
	}
	r, err := terraform.NewRunner(cfg)
	require.NoError(t, err)
	return r, dir
}

func minimalSpec() repository.VMSpec {
	return repository.VMSpec{Name: "vm", CPUCores: 1, MemoryMB: 512, DiskGB: 4, OSType: "l26"}
}

func TestNewRunner_CreatesWorkdirAndPluginCache(t *testing.T) {
	_, dir := newTestRunner(t, false)
	assert.DirExists(t, filepath.Join(dir, "work"))
	assert.DirExists(t, filepath.Join(dir, "work", ".plugin-cache"))
}

func TestProvision_WritesMainTFAndReturnsVMID(t *testing.T) {
	r, dir := newTestRunner(t, false)
	id := uuid.New()

	vmid, err := r.Provision(context.Background(), id, minimalSpec())

	require.NoError(t, err)
	assert.Equal(t, 123, vmid)
	assert.FileExists(t, filepath.Join(dir, "work", id.String(), "main.tf"))

	calls, _ := os.ReadFile(filepath.Join(dir, "calls.log"))
	assert.Contains(t, string(calls), "init")
	assert.Contains(t, string(calls), "apply")
	assert.Contains(t, string(calls), "output")
}

func TestProvision_ApplyFailsCleansWorkspace(t *testing.T) {
	r, dir := newTestRunner(t, true)
	id := uuid.New()

	_, err := r.Provision(context.Background(), id, minimalSpec())

	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(dir, "work", id.String()))
	calls, _ := os.ReadFile(filepath.Join(dir, "calls.log"))
	assert.Contains(t, string(calls), "destroy")
}

func TestDeprovision_MissingWorkspace(t *testing.T) {
	r, _ := newTestRunner(t, false)
	err := r.Deprovision(context.Background(), uuid.New())
	assert.ErrorIs(t, err, repository.ErrWorkspaceNotFound)
}

func TestDeprovision_RemovesWorkspace(t *testing.T) {
	r, dir := newTestRunner(t, false)
	id := uuid.New()
	_, err := r.Provision(context.Background(), id, minimalSpec())
	require.NoError(t, err)

	require.NoError(t, r.Deprovision(context.Background(), id))
	assert.NoDirExists(t, filepath.Join(dir, "work", id.String()))
}

func TestProvision_DoesNotWriteCredentialsToFile(t *testing.T) {
	r, dir := newTestRunner(t, false)
	id := uuid.New()
	_, err := r.Provision(context.Background(), id, minimalSpec())
	require.NoError(t, err)

	tf, _ := os.ReadFile(filepath.Join(dir, "work", id.String(), "main.tf"))
	assert.NotContains(t, string(tf), "secret")
	assert.NotContains(t, string(tf), "user@pam")
}
