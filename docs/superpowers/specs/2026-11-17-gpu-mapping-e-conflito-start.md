# GPU por resource mapping e bloqueio de start em conflito

Data: 2026-11-17 · Status: aprovada para execução · Substitui o trecho de GPU da spec
`2026-09-07-terraform-provisioning-design.md` (seção do template, `GPUPCI`).

## 1. Contexto

O nó `pve01` tem duas RTX 4060, expostas como resource mappings do Proxmox:
`gpu-rtx4060-01` (slot 1, PCIe da CPU) e `gpu-rtx4060-02` (slot 2, PCIe do chipset).
Hoje o backend conhece uma só, via `PROXMOX_GPU_PCI`, e a criação de VM com GPU falha.

Dois problemas:

1. **Bug**: criar VM com GPU devolve 500. O Proxmox responde
   `400 hostpci0: invalid format - hostpci0.host: value does not match the regex pattern`
   e o Terraform desfaz o clone.
2. **Funcionalidade**: usar as duas placas e impedir que uma VM seja ligada quando a GPU dela
   já está em uso por outra VM ligada.

## 2. Causa raiz do bug

- `backend/.env`: `PROXMOX_GPU_PCI=mapping=gpu-rtx4060-02,pcie=1`.
- `internal/terraform/template.go:78-80` corta na vírgula e entrega `mapping=gpu-rtx4060-02`.
- `internal/terraform/main.tf.tmpl:104-110` grava isso em `id = "..."`. No provider bpg, `id` vira o
  parâmetro `host` da API, que só aceita endereço PCI (`0000:01:00.0`).
- O atributo certo para mapping é `mapping = "<nome>"` no bloco `hostpci`.

Fato adicional confirmado na documentação do provider (`bpg/proxmox`, bloco `hostpci`): **`id` não
é compatível com `api_token`**; exige usuário root com senha. Como o ProxLab só autentica por
token, endereço PCI cru nunca funcionaria. Isso muda uma sugestão do briefing (ver D1).

## 3. Regras de negócio (dadas, não reabrir)

1. Aluno e pesquisador podem pedir GPU.
2. Criar VM com GPU é sempre permitido, mesmo com a GPU em uso. Várias VMs podem ter a mesma GPU anexada.
3. Ligar uma VM com GPU é bloqueado se outra VM ligada usa o mesmo mapping; a VM continua desligada.
4. VM sem GPU funciona exatamente como hoje, sem verificação nova.
5. O front explica por que não dá para ligar.
6. Erro interno do Proxmox nunca chega ao usuário.

## 4. Decisões fechadas

**D1. Config só aceita nomes de mapping.**
`PROXMOX_GPU_MAPPINGS=gpu-rtx4060-01,gpu-rtx4060-02` (lista, ordem = prioridade no desempate).
Endereço PCI cru é rejeitado na carga da config, com erro claro, porque o provider não o aceita
com token (seção 2). Compatibilidade: se `PROXMOX_GPU_MAPPINGS` estiver vazia e `PROXMOX_GPU_PCI`
existir no formato `mapping=<nome>[,opções]`, vira lista de um item. `pcie = true` continua fixo.

**D2. O backend atribui a GPU na criação, balanceando.**
Escolhe o mapping com menos VMs anexadas (contagem no banco); empate → ordem da lista.
Motivo: com passthrough a placa fica gravada na config da VM; decidir só ao ligar exigiria
reescrever `hostpci0` via API a cada start (permissão extra, state do Terraform divergente).
Escolha pelo usuário foi descartada: ele não tem como saber qual está livre, e o balanceamento
já reduz conflito.

**D3. Associação persistida em `vms.gpu_mapping`** (migration 005, `VARCHAR(64) NOT NULL DEFAULT ''`).
É um dado de configuração da VM (não muda), não um estado. A *ocupação* não é persistida: é
lida do Proxmox a cada start, como o status de containers.

**D4. VM com GPU nasce desligada** (`started = false`, status `stopped`), inclusive clone Ubuntu.
Motivo: o clone Ubuntu hoje nasce ligado; se a GPU estiver ocupada o start falha dentro do
`terraform apply`, o provider faz rollback e a criação devolve 500, violando a regra 2. Nascendo
desligada, a criação nunca disputa a placa e todo start passa pelo mesmo caminho verificado
(regra 3). Custo: um clique em "Ligar máquina". O cloud-init roda no primeiro boot normalmente
e as credenciais continuam vindo no `201`.

**D5. "Em uso" = status do Proxmox diferente de `stopped`** (o endpoint `status/current` devolve
`running` também para pausada/suspensa: o processo QEMU existe e segura a placa). Start em
andamento é coberto por D7.

**D6. Ocupação verificada em duas camadas.**
- *Pré-checagem* (rápida, mensagem imediata): VMs do banco com o mesmo `gpu_mapping`, exceto a
  própria; para cada uma, `GetVMStatus`. Erro ao consultar uma delas (ex.: VM apagada à mão no
  Proxmox) → loga e não conta como ocupada.
- *Rede de segurança* (autoritativa): o Proxmox recusa o start de qualquer forma. **A recusa não
  vem na resposta HTTP do `POST .../status/start`** (que só devolve o UPID); vem no resultado da
  tarefa. Por isso o start de VM com GPU passa a esperar a tarefa (`WaitTask`) e, se o
  `exitstatus` não for `OK`, classifica como conflito quando (a) refazendo a pré-checagem aparece
  uma VM ocupando, ou (b) o `exitstatus` contém `already in use`. Cobre VMs fora do ProxLab,
  VMs legadas e corrida.

**D7. Starts de VM com GPU são serializados** por um `sync.Mutex` no `VMUseCase` (pré-checagem +
`StartVM` + `WaitTask`). Assim o segundo start já enxerga o primeiro como ligado. Pressupõe uma
instância do backend (é o caso).

**D8. Sem indicador "GPU livre/ocupada" na v1.** O indicador seria só consultivo (corrida, VMs
externas invisíveis à pré-checagem) e custaria chamadas ao Proxmox em todo `GET /api/vms/:id`.
O clique em Ligar dá a resposta autoritativa em segundos. O front passa a mostrar *qual* GPU a
VM tem. Indicador fica como melhoria futura.

**D9. VMs legadas** (`require_gpu = true` e `gpu_mapping = ''`): não entram na pré-checagem nem
no balanceamento, mas o start delas passa pelo mutex + `WaitTask` + tradução do erro, então a
mensagem de conflito também aparece. Sem backfill automático. Opcional para o admin, após
conferir `hostpci0` no Proxmox:
`UPDATE vms SET gpu_mapping = '<mapping>' WHERE id = '<uuid>';`

**D10. GPU pedida sem mapping configurado → 422** com mensagem clara. Hoje a VM é criada sem
GPU e gravada como `require_gpu = true`, o que engana o usuário.

**D11. Falta de `Mapping.Use` vira erro próprio.** Se o `terraform apply` falhar e a saída
contiver `Mapping.Use`, o Runner devolve `repository.ErrGPUNotPermitted` → 503 com mensagem
amigável e log explícito para o admin. A correção é ACL no Proxmox, fora do código.

## 5. Mudanças por camada

### Config (`internal/config/config.go`)
- Remove `ProxmoxGPUPCI`; adiciona `ProxmoxGPUMappings []string`.
- `parseGPUMappings(mappings, legacy string) ([]string, error)`: separa por vírgula, `TrimSpace`,
  ignora vazios, remove duplicados preservando a ordem; cada item deve casar
  `^[A-Za-z][A-Za-z0-9_-]*$`. Fallback legado conforme D1. Erro → `Load` falha.
- Novo `config_test.go` cobrindo os casos da seção 7.

### Domínio
- `entity.VM`: `GPUMapping string` (`gorm:"column:gpu_mapping;type:varchar(64);not null;default:''" json:"gpu_mapping"`).
- `repository.VMSpec`: sai `RequireGPU bool`, entra `GPUMapping string` (vazio = sem GPU).
- `repository.ErrGPUNotPermitted` (em `provisioner.go`).
- `VMRepository`: `FindWithGPU(ctx) ([]entity.VM, error)` (todas com `gpu_mapping <> ''`); serve
  ao balanceamento e à pré-checagem.
- `ProxmoxClientInterface`: `WaitTask(ctx, upid string) (exitStatus string, err error)`.

### Terraform (`internal/terraform`)
- `Render(spec, node)`: some o parâmetro `gpuPCI`; `templateData.GPUMapping`.
- `main.tf.tmpl`:
  ```hcl
  hostpci {
    device  = "hostpci0"
    mapping = "{{ hcl .GPUMapping }}"
    pcie    = true
  }
  ```
  e, no ramo clone, `started = false` quando `.GPUMapping` não é vazio (senão `true`, como hoje).
- `Runner`: some o campo `gpuPCI`; em falha de `Apply`, se o erro contiver `Mapping.Use`,
  embrulha `repository.ErrGPUNotPermitted` (o destroy best-effort e a limpeza continuam).

### Proxmox (`internal/proxmox`)
- `WaitTask`: `GET /nodes/{node}/tasks/{url.PathEscape(upid)}/status` a cada 1 s (intervalo em
  campo não exportado, para teste) até `status == "stopped"`; devolve `exitstatus`. Respeita o
  `ctx` (o usecase dá 90 s).
- Tipo privado `pveTaskStatus{Status, ExitStatus}`.

### Repository + migration
- `vm_repository.go`: `FindWithGPU`.
- `migrations/005_add_vm_gpu_mapping.sql`:
  `ALTER TABLE vms ADD COLUMN IF NOT EXISTS gpu_mapping VARCHAR(64) NOT NULL DEFAULT '';`
  (em desenvolvimento o AutoMigrate cria a coluna).

### UseCase (`internal/usecase/vm_usecase.go`)
- `NewVMUseCase` recebe `gpuMappings []string`.
- Erros novos: `ErrGPUInUse`, `ErrGPUUnavailable`, `ErrVMStartFailed`.
- `CreateVM`, quando `req.RequireGPU`:
  1. sem mappings → `ErrGPUUnavailable`;
  2. `pickGPUMapping`: conta por mapping em `FindWithGPU`, menor contagem, empate pela ordem da config
     (mappings que saíram da config são ignorados na contagem);
  3. `spec.GPUMapping = escolhido`; se for clone, status inicial `stopped`;
  4. grava `vm.GPUMapping`.
  `ErrGPUNotPermitted` do provisioner sobe embrulhado (`%w`).
- `StartVM`:
  - `!vm.RequireGPU` → caminho atual, intocado.
  - com GPU: `gpuMu.Lock()`; se `vm.GPUMapping != ""`, pré-checagem → `ErrGPUInUse`;
    `StartVM`; `WaitTask` com timeout de 90 s (erro/timeout → erro genérico, status do banco não
    muda); `exit != "OK"` → loga o `exitstatus` (só no servidor) e devolve `ErrGPUInUse`
    conforme D6, senão `ErrVMStartFailed`; sucesso → `UpdateStatus(running)`.
- `RebootVM`, `StopVM`, `DeleteVM`, `GetVM`: sem mudança.

### Handler (`internal/handler/vm_handler.go`)
- `main.go` só repassa `cfg.ProxmoxGPUMappings` ao construtor.
- Mapeamento de erros (seção 6) em `Create` e em `handleVMError`. Payload inalterado
  (`require_gpu` continua booleano; o cliente não escolhe a placa).

### Frontend
O botão "Ligar máquina" fica em `maquinas-virtuais/[id]/page.tsx` (detalhes), não em `utilizar`.
O erro do backend já aparece ali via `PageNotice kind="error"` (o `request` de `lib/api.ts`
propaga `body.error`), então a mensagem de conflito não exige componente novo.
- `lib/api.ts`: `gpu_mapping: string` em `VM`.
- `[id]/page.tsx` e `[id]/utilizar/page.tsx`: campo GPU mostra `sim (gpu-rtx4060-01)` quando
  houver mapping, `sim` para legadas, `não` caso contrário.
- `criar/page.tsx`: hint da GPU deixa de dizer "perfil pesquisador, uma máquina por vez" →
  "A máquina é criada desligada. Só liga se a GPU não estiver em uso por outra máquina.";
  no modal de credenciais, quando a VM foi criada com GPU, uma linha com `FORM.hint` avisando
  que ela está desligada e deve ser ligada na tela de detalhes.
- Nenhuma classe/estilo novo; só `@/components` e `FORM.*` (`docs/frontend-design-system.md`).

### Smoke (`cmd/proxmox-check`)
- `--gpu[=<mapping>]`: cria VM de teste com o mapping (padrão: primeiro da config), confere
  `stopped`, destrói. Valida o fix e a ACL sem disputar a placa.
- `--gpu-conflict[=<mapping>]`: cria duas VMs de teste no mesmo mapping; liga A e espera a
  tarefa; liga B e imprime o `exitstatus`; para A; destrói as duas (sempre, via `defer`).
  Se A já falhar por a placa estar em uso por uma VM real, isso é evidência válida do conflito:
  imprime e segue para a limpeza.

## 6. Mapeamento de erros HTTP

| Situação | Erro | HTTP | Mensagem ao usuário |
|---|---|---|---|
| Start com GPU em uso (pré-checagem ou recusa do Proxmox) | `ErrGPUInUse` | 409 | "A GPU desta máquina está em uso por outra máquina virtual. Tente ligar novamente quando ela for desligada." |
| GPU pedida, nenhum mapping configurado | `ErrGPUUnavailable` | 422 | "GPU não está disponível nesta instalação." |
| Token sem `Mapping.Use` na criação | `ErrGPUNotPermitted` | 503 | "GPU temporariamente indisponível. Contate o administrador." |
| Tarefa de start falhou por outro motivo | `ErrVMStartFailed` | 500 | "Não foi possível ligar a máquina virtual." |
| Demais falhas | — | 500 | mensagens genéricas atuais |

Nenhuma resposta carrega texto do Proxmox ou do Terraform; o detalhe vai só para o log.

## 7. Casos de teste

**config**: lista com dois; espaços e duplicados; vazia; fallback `mapping=x,pcie=1`; fallback com
endereço cru → erro; nome inválido → erro; lista nova tem precedência sobre a legada.

**terraform/template**: com GPU → `mapping = "gpu-rtx4060-01"`, `pcie = true`, sem `id =`, HCL
válido; clone + GPU → `started       = false`; clone sem GPU → `started       = true`; sem GPU →
sem `hostpci`. (Substituem `TestRender_WithGPU` e `TestRender_GPURequestedButNotConfigured`.)

**proxmox/client** (httptest): `WaitTask` devolve `OK`; devolve `exitstatus` de erro; segue
consultando enquanto `running`; respeita cancelamento do `ctx`; UPID escapado no path.

**usecase – criação**: GPU sem mappings → `ErrGPUUnavailable`, sem `Provision`; primeira VM →
primeiro mapping; balanceia (1 em 01, 0 em 02 → 02); empate → ordem; mapping fora da config é
ignorado; clone Ubuntu com GPU → status `stopped` e credenciais presentes; sem GPU →
`spec.GPUMapping == ""` e status `running` como hoje; `ErrGPUNotPermitted` propagado.

**usecase – start**: sem GPU → não chama `FindWithGPU` nem `WaitTask`; GPU livre → `running`;
outra VM `running` no mesmo mapping → `ErrGPUInUse`, sem `StartVM`; outra VM em mapping
diferente → liga; a própria VM não conta; erro no `GetVMStatus` da outra → não bloqueia;
tarefa com `already in use` → `ErrGPUInUse`, status não muda; tarefa com outro erro →
`ErrVMStartFailed`; legada (sem mapping) → pula pré-checagem, espera a tarefa, traduz o erro.

**handler**: 409/422/503 com as mensagens da seção 6 (se a suíte de handler de VM não existir,
cobrir por teste de tabela de `handleVMError`).

## 8. Riscos

- **ACL `Mapping.Use`**: o token precisa de `Mapping.Use` em `/mapping/pci/<mapping>` para *cada*
  mapping. Sintoma: criação falha com `403 ... Mapping.Use`. Correção pelo admin, por mapping:
  `pveum acl modify /mapping/pci/gpu-rtx4060-01 --tokens '<token-id>' --roles PVEMappingUser`
  (token com privilégios separados: o usuário dono também precisa do papel). O executor não
  contorna: reporta qual mapping falhou.
- **`pcie = true` exige máquina q35**: o template Ubuntu (VMID 9000) precisa ser q35. Se a
  criação falhar com erro de `pcie`/`q35`, parar e reportar; não alterar o template.
- **Texto do `exitstatus`**: `already in use` é o esperado, mas precisa ser confirmado no teste
  real (`--gpu-conflict`). A classificação (a) da D6 independe dele.
  **Confirmado em 2026-11-17** (`--gpu-conflict=gpu-rtx4060-01`): a tarefa de start termina com
  `exitstatus = "PCI device '0000:01:00.0' already in use by VMID '101'"` (a placa estava em uso por
  uma VM real, VMID 101). O texto contém `already in use`; a detecção (b) está correta. O ramo B
  (segunda VM de teste) não chegou a rodar porque a A já foi recusada; a limpeza destruiu as duas VMs de teste.
- **Corrida**: entre backend e ações feitas direto no Proxmox, quem decide é o Proxmox; o
  usuário recebe a mesma mensagem de conflito.
- **VMs fora do ProxLab / legadas** usando a placa: invisíveis à pré-checagem, cobertas pela
  rede de segurança.
- **Reboot**: a placa é liberada e retomada pelo próprio Proxmox; janela desprezível, sem tratamento.
- **Start mais lento**: VM com GPU só responde depois que a tarefa termina (segundos; teto 90 s).
- **Mapping removido da config** com VMs associadas: elas continuam funcionando; só saem do balanceamento.

## 9. Validação ponta a ponta (sem tocar em VMs existentes)

Tudo aqui cria e apaga apenas VMs do próprio teste, e só roda com confirmação do autor.

1. `go run ./cmd/proxmox-check` — regressão do fluxo sem GPU.
2. `go run ./cmd/proxmox-check --gpu=gpu-rtx4060-01` e `--gpu=gpu-rtx4060-02` — prova o fix do
   bug e a ACL de cada mapping. VM nasce desligada: não disputa a placa.
3. `go run ./cmd/proxmox-check --gpu-conflict=<mapping>` — prova a recusa e captura o
   `exitstatus`. Ocupa a placa por alguns segundos se ela estiver livre; combinar o horário.
4. Pelo front (feito pelo autor): criar VM com GPU como aluno e como pesquisador (nascem
   desligadas, em placas diferentes); criar uma terceira (cai na mesma placa da primeira); ligar
   a primeira; tentar ligar a terceira → mensagem de conflito, VM segue desligada; desligar a
   primeira; ligar a terceira → liga; excluir as três.
5. VM sem GPU: criar, ligar, desligar, excluir — comportamento idêntico ao atual.

## 10. Fases

| Fase | Entrega | Verificação |
|---|---|---|
| 1 | Fix do formato: config em lista, `VMSpec.GPUMapping`, template com `mapping`, VM com GPU nasce desligada, usa o primeiro mapping, erros 422/503, smoke `--gpu` | `go test ./...`, `go build ./...`, validação 1 e 2 |
| 2 | Coluna `gpu_mapping`, migration 005, `FindWithGPU`, balanceamento, campo no front | `go test ./...`, `npm run lint`, `npm run build` |
| 3 | `WaitTask`, mutex, pré-checagem, tradução do erro, 409, textos do front, smoke `--gpu-conflict` | testes da seção 7, validação 3 a 5 |
| 4 | `CLAUDE.md`, spec de 2026-09-07 (trecho `GPUPCI`), `.env.example`, swagger se aplicável | revisão do diff |

## 11. Fora de escopo

Indicador de disponibilidade, fila/agendamento de GPU, escolha manual da placa, mais de uma GPU
por VM, troca de placa de VM existente, backfill automático de VMs legadas.
