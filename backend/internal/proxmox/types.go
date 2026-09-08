package proxmox

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// VMStatus representa o estado de runtime de uma VM obtido via
// GET /nodes/{node}/qemu/{vmid}/status/current
type VMStatus struct {
	VMID    int
	Name    string
	Status  string  // "running" | "stopped" | "paused"
	CPU     float64 // fração de uso de CPU (0.0–1.0)
	MemUsed int64   // memória em uso (bytes)
	MemMax  int64   // memória total alocada (bytes)
	Uptime  int64   // tempo ligada em segundos
}

// TermProxyResponse contém as credenciais retornadas por
// POST /nodes/{node}/qemu/{vmid}/termproxy
// Usado para abrir um terminal xterm.js via WebSocket.
type TermProxyResponse struct {
	Ticket string // ticket de autenticação temporário
	Port   int    // porta do WebSocket de terminal no nó Proxmox
	UPID   string // UPID da tarefa
}

// --- Tipos privados para decodificação do JSON da API do Proxmox ---

// pveEnvelope é o wrapper JSON de todas as respostas da API: {"data": T}
type pveEnvelope[T any] struct {
	Data T `json:"data"`
}

type pveVMStatus struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
	Uptime int64   `json:"uptime"`
}

type pveTermProxy struct {
	Ticket string  `json:"ticket"`
	Port   flexInt `json:"port"`
	UPID   string  `json:"upid"`
}

// flexInt deserializa um campo JSON que pode chegar como número (5900)
// ou como string ("5900") dependendo da versão do Proxmox.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("flexInt: não é número nem string: %s", data)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("flexInt: string não numérica: %q", s)
	}
	*f = flexInt(n)
	return nil
}
