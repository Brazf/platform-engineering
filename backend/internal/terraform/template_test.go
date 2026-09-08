package terraform_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseSpec() repository.VMSpec {
	return repository.VMSpec{
		Name:     "minha-vm",
		CPUCores: 2,
		MemoryMB: 2048,
		DiskGB:   20,
		OSType:   "l26",
	}
}

// parseHCL garante que a saída é HCL sintaticamente válido.
func parseHCL(t *testing.T, src []byte) {
	t.Helper()
	_, diags := hclparse.NewParser().ParseHCL(src, "main.tf")
	require.False(t, diags.HasErrors(), "HCL inválido: %s\n---\n%s", diags.Error(), src)
}

func TestRender_MinimalSpec(t *testing.T) {
	out, err := terraform.Render(baseSpec(), "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `source  = "bpg/proxmox"`)
	assert.Contains(t, s, `node_name     = "pve01"`)
	assert.Contains(t, s, `name          = "minha-vm"`)
	assert.Contains(t, s, `started       = false`)
	assert.Contains(t, s, `cores = 2`)
	assert.Contains(t, s, `dedicated = 2048`)
	assert.Contains(t, s, `datastore_id = "local-zfs"`)
	assert.Contains(t, s, `size         = 20`)
	assert.Contains(t, s, `bridge = "vmbr0"`)
	assert.Contains(t, s, `type = "l26"`)
	assert.Contains(t, s, `output "vmid"`)
	assert.NotContains(t, s, "pool_id")
	assert.NotContains(t, s, "tags")
	assert.NotContains(t, s, "cdrom")
	assert.NotContains(t, s, "hostpci")
	assert.NotContains(t, s, "vm_id ")
}

func TestRender_WithISO(t *testing.T) {
	spec := baseSpec()
	spec.ISO = "local:iso/ubuntu-22.04.iso"
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `file_id   = "local:iso/ubuntu-22.04.iso"`)
	assert.Contains(t, s, `interface = "ide2"`)
	assert.Contains(t, s, `boot_order = ["ide2", "scsi0"]`)
}

func TestRender_WithGPU(t *testing.T) {
	spec := baseSpec()
	spec.RequireGPU = true
	out, err := terraform.Render(spec, "pve01", "0000:01:00.0,pcie=1")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `device = "hostpci0"`)
	assert.Contains(t, s, `id     = "0000:01:00.0"`)
	assert.Contains(t, s, `pcie   = true`)
}

func TestRender_GPURequestedButNotConfigured(t *testing.T) {
	spec := baseSpec()
	spec.RequireGPU = true
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	assert.NotContains(t, string(out), "hostpci")
}

func TestRender_WithPoolAndTags(t *testing.T) {
	spec := baseSpec()
	spec.Pool = "students"
	spec.Tags = []string{"proxlab", "aluno"}
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `pool_id       = "students"`)
	assert.Contains(t, s, `tags          = ["proxlab", "aluno"]`)
}

func TestRender_EscapesQuotesAndInterpolation(t *testing.T) {
	spec := baseSpec()
	spec.Name = `vm "x" ${evil}`
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	assert.True(t, strings.Contains(string(out), `name          = "vm \"x\" $${evil}"`), string(out))
}

func TestRender_CustomStorageAndBridge(t *testing.T) {
	spec := baseSpec()
	spec.Storage = "local-lvm"
	spec.Bridge = "vmbr1"
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	assert.Contains(t, string(out), `datastore_id = "local-lvm"`)
	assert.Contains(t, string(out), `bridge = "vmbr1"`)
}

func TestRender_CloneMode(t *testing.T) {
	spec := baseSpec()
	spec.CloneVMID = 9000
	spec.CloudInitUser = "ubuntu"
	spec.CloudInitPassword = `s3nh@"x`
	spec.ISO = "local:iso/ignored.iso"
	spec.Pool = "students"
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, "clone {")
	assert.Contains(t, s, `vm_id = 9000`)
	assert.Contains(t, s, `full  = true`)
	assert.Contains(t, s, `started       = true`)
	assert.Contains(t, s, `username = "ubuntu"`)
	assert.Contains(t, s, `password = "s3nh@\"x"`)
	assert.Contains(t, s, `address = "dhcp"`)
	assert.Contains(t, s, `discard      = "on"`)
	assert.Contains(t, s, "agent {")
	assert.Contains(t, s, "enabled = false")
	assert.Contains(t, s, `pool_id       = "students"`)
	assert.NotContains(t, s, "cdrom")
	assert.NotContains(t, s, "operating_system")
	assert.NotContains(t, s, "machine")
	assert.NotContains(t, s, "scsi_hardware")
}

func TestRender_NoCloneHasNoInitialization(t *testing.T) {
	out, err := terraform.Render(baseSpec(), "pve01", "")
	require.NoError(t, err)
	s := string(out)
	assert.NotContains(t, s, "clone {")
	assert.NotContains(t, s, "initialization")
	assert.NotContains(t, s, "agent {")
}
