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
	Node       string
	Name       string
	CPUCores   int
	MemoryMB   int
	DiskGB     int
	Storage    string
	OSType     string
	ISO        string
	Bridge     string
	Pool       string
	Tags       []string
	GPUMapping string // vazio = sem passthrough

	CloneVMID         int
	CloudInitUser     string
	CloudInitPassword string
	CloudInitSSHKeys  []string
	StaticIP          string
	Gateway           string
}

// Render gera o main.tf para a spec. spec.GPUMapping (vazio = sem GPU) vira um
// bloco hostpci com `mapping`; VM clonada com GPU nasce desligada.
func Render(spec repository.VMSpec, node string) ([]byte, error) {
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

		GPUMapping: spec.GPUMapping,

		CloneVMID:         spec.CloneVMID,
		CloudInitUser:     spec.CloudInitUser,
		CloudInitPassword: spec.CloudInitPassword,
		CloudInitSSHKeys:  spec.CloudInitSSHKeys,
		StaticIP:          spec.StaticIP,
		Gateway:           spec.Gateway,
	}
	if data.Storage == "" {
		data.Storage = defaultStorage
	}
	if data.Bridge == "" {
		data.Bridge = defaultBridge
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
