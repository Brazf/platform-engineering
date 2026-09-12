package usecase

import "fmt"

// RoleLimits são os tetos por perfil, compartilhados por VMs e containers.
type RoleLimits struct {
	MaxCPU        int // cores (VM) ou CPU inteira (container: MaxCPU*1000 millis)
	MaxMemMB      int
	MaxContainers int
	MaxStorageGB  int // soma dos volumes persistentes
}

var roleLimits = map[string]RoleLimits{
	"aluno":       {MaxCPU: 2, MaxMemMB: 4096, MaxContainers: 5, MaxStorageGB: 20},
	"pesquisador": {MaxCPU: 8, MaxMemMB: 16384, MaxContainers: 20, MaxStorageGB: 100},
}

// LimitsFor devolve os limites do perfil; role desconhecida cai em aluno por segurança.
func LimitsFor(role string) RoleLimits {
	if l, ok := roleLimits[role]; ok {
		return l
	}
	return roleLimits["aluno"]
}

func validateRoleLimits(role string, cpuCores, memoryMB int) error {
	limits := LimitsFor(role)
	if cpuCores > limits.MaxCPU {
		return fmt.Errorf("%w: perfil '%s' permite no máximo %d CPU(s)", ErrLimitExceeded, role, limits.MaxCPU)
	}
	if memoryMB > limits.MaxMemMB {
		return fmt.Errorf("%w: perfil '%s' permite no máximo %dMB de RAM", ErrLimitExceeded, role, limits.MaxMemMB)
	}
	return nil
}
