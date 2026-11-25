package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGPUMappings(t *testing.T) {
	tests := []struct {
		name      string
		mappings  string
		legacy    string
		want      []string
		wantError bool
	}{
		{name: "lista com dois", mappings: "gpu-rtx4060-01,gpu-rtx4060-02", want: []string{"gpu-rtx4060-01", "gpu-rtx4060-02"}},
		{name: "espaços e vazios", mappings: " gpu-a , ,gpu-b ", want: []string{"gpu-a", "gpu-b"}},
		{name: "duplicados preservam ordem", mappings: "gpu-b,gpu-a,gpu-b", want: []string{"gpu-b", "gpu-a"}},
		{name: "vazia", want: nil},
		{name: "fallback legado", legacy: "mapping=gpu-rtx4060-02,pcie=1", want: []string{"gpu-rtx4060-02"}},
		{name: "fallback legado sem opções", legacy: "mapping=gpu-x", want: []string{"gpu-x"}},
		{name: "fallback legado com endereço cru", legacy: "0000:01:00.0,pcie=1", wantError: true},
		{name: "endereço cru na lista nova", mappings: "0000:01:00.0", wantError: true},
		{name: "nome inválido", mappings: "gpu a", wantError: true},
		{name: "nome inválido com ponto", mappings: "gpu.a", wantError: true},
		{name: "lista nova tem precedência", mappings: "gpu-novo", legacy: "mapping=gpu-velho", want: []string{"gpu-novo"}},
		{name: "legado ignorado se lista nova presente mesmo inválido", mappings: "gpu-novo", legacy: "0000:01:00.0", want: []string{"gpu-novo"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGPUMappings(tc.mappings, tc.legacy)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
