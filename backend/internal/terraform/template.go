package terraform

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/proxlab/backend/internal/domain/repository"
)

//go:embed main.tf.tmpl
var mainTemplateSrc string

var mainTemplate = template.Must(
	template.New("main.tf").Funcs(template.FuncMap{"hcl": hclString}).Parse(mainTemplateSrc),
)

const (
	defaultStorage = "local-zfs"
	defaultBridge  = "vmbr0"
)

// templateData é o que o template enxerga, já com defaults aplicados.
type templateData struct {
	Node     string
	Name     string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Storage  string
	OSType   string
	ISO      string
	Bridge   string
	Pool     string
	Tags     []string
	GPUPCI   string // vazio = sem passthrough
}

// Render gera o main.tf para a spec. gpuPCI é o valor de PROXMOX_GPU_PCI
// (ex: "0000:01:00.0,pcie=1"); só a parte antes da vírgula é usada, e apenas
// quando spec.RequireGPU é true.
func Render(spec repository.VMSpec, node, gpuPCI string) ([]byte, error) {
	data := templateData{
		Node:     node,
		Name:     spec.Name,
		CPUCores: spec.CPUCores,
		MemoryMB: spec.MemoryMB,
		DiskGB:   spec.DiskGB,
		Storage:  spec.Storage,
		OSType:   spec.OSType,
		ISO:      spec.ISO,
		Bridge:   spec.Bridge,
		Pool:     spec.Pool,
		Tags:     spec.Tags,
	}
	if data.Storage == "" {
		data.Storage = defaultStorage
	}
	if data.Bridge == "" {
		data.Bridge = defaultBridge
	}
	if spec.RequireGPU && gpuPCI != "" {
		data.GPUPCI, _, _ = strings.Cut(gpuPCI, ",")
	}

	var buf bytes.Buffer
	if err := mainTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("erro ao renderizar template terraform: %w", err)
	}
	return buf.Bytes(), nil
}

// hclString escapa um valor para uso dentro de uma string HCL entre aspas.
// "${" vira "$${" para não ser interpretado como interpolação.
func hclString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "${", "$${")
	s = strings.ReplaceAll(s, "%{", "%%{")
	return s
}
