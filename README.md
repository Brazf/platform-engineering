# ProxLab

Plataforma web para criação e gerenciamento de Máquinas Virtuais no Proxmox VE, desenvolvida para alunos e pesquisadores do IFPE.

## Pré-requisitos

- Docker 24+ e Docker Compose v2
- Acesso à rede interna da faculdade (onde está o servidor Proxmox)
- Token de API do Proxmox com as permissões necessárias

---

## Deploy no servidor da faculdade

### 1. Clonar o repositório

```bash
git clone <url-do-repositorio>
cd platform-tcc
```

### 2. Configurar variáveis de ambiente

```bash
cp .env.example .env
```

Edite `.env` com os valores reais:

```bash
nano .env
```

Campos obrigatórios:

| Variável | Descrição |
|---|---|
| `DB_PASSWORD` | Senha para o PostgreSQL (escolha uma senha forte) |
| `JWT_SECRET` | Segredo para assinar tokens JWT (`openssl rand -hex 32`) |
| `PROXMOX_TOKEN_ID` | Token no formato `usuario@realm!nome-do-token` |
| `PROXMOX_TOKEN_SECRET` | Segredo gerado pelo Proxmox ao criar o token |

### 3. Configurar o token da API do Proxmox

No painel do Proxmox (`https://pve01:8006`):

1. **Criar o token:** `Datacenter → Permissions → API Tokens → Add`
   - User: `fabio.ferreira@pve` (ou outro usuário com permissões)
   - Token ID: `backend-api`
   - Desmarque "Privilege Separation" para herdar permissões do usuário

2. **Permissões mínimas necessárias** (em `Datacenter → Permissions → Add`):
   - Path `/` → Role `PVEAuditor` — leitura geral do cluster
   - Path `/nodes/pve01` → Role `PVEVMAdmin` — criar/gerenciar VMs
   - Path `/storage/local-zfs` → Role `PVEDatastoreUser` — alocar discos
   - Path `/pool/researchers` → Role `PVEVMUser` — adicionar VMs ao pool
   - Path `/pool/students` → Role `PVEVMUser`

3. Copie o **Token ID** (`fabio.ferreira@pve!backend-api`) e o **Secret** gerado para o `.env`.

### 4. Subir os serviços

```bash
docker compose up -d --build
```

Verificar se todos os serviços estão saudáveis:

```bash
docker compose ps
```

A aplicação estará disponível em `http://<ip-do-servidor>`.

### 5. Executar migrations (produção)

O backend aplica `auto-migrate` apenas quando `APP_ENV=development`. Em produção, rode as migrations manualmente:

```bash
# Conectar ao container do banco
docker compose exec postgres psql -U proxlab -d proxlab

# Ou aplicar um script SQL específico
docker compose exec -T postgres psql -U proxlab -d proxlab < backend/migrations/001_initial.sql
```

Os scripts de migration estão em `backend/migrations/`.

---

## Desenvolvimento local (sem Docker)

### Backend

```bash
cd backend
cp ../.env.example .env   # ajuste DB_HOST=localhost
go run ./cmd/server
```

### Frontend

```bash
cd frontend
npm install
npm run dev   # inicia em http://localhost:3000
```

> Em dev, o frontend em `localhost:3000` não consegue alcançar o backend em `localhost:8080` pelo nginx. Use o Docker Compose ou configure um proxy no sistema operacional.

### Túnel SSH para o cluster k3s (containers)

Pela VPN do laboratório só a porta 22 chega à VM `k3s-server` (172.26.3.250). Sem o túnel, as rotas de containers falham com
`dial tcp 172.26.3.250:6443: i/o timeout`. Faça isso **toda vez** que ligar o notebook/VPN ou o túnel cair:

```bash
# 1. Abre o túnel em segundo plano: localhost:6443 → API do k3s
ssh -f -N -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 \
  -L 6443:127.0.0.1:6443 \
  -i ~/.proxlab/terraform/k3s/id_ed25519 ubuntu@172.26.3.250

# 2. Confere: deve imprimir 401 (API respondeu, sem credencial)
curl -sk -o /dev/null -w "%{http_code}\n" https://127.0.0.1:6443/version
```

O `backend/.env` precisa apontar para o kubeconfig que usa `127.0.0.1` (só em dev):

```
KUBECONFIG_PATH=/Users/<você>/.proxlab/terraform/k3s/kubeconfig-tunnel
```

Depois reinicie o backend (`go run ./cmd/server`), pois o cliente do k3s é criado na inicialização.

- Ver se o túnel está aberto: `lsof -iTCP:6443 -sTCP:LISTEN`
- Fechar o túnel: `pkill -f "6443:127.0.0.1:6443"`
- Em produção (backend dentro da rede do laboratório) não há túnel: `KUBECONFIG_PATH` usa o kubeconfig original, com `172.26.3.250:6443`.

---

## Comandos úteis

```bash
# Ver logs do backend em tempo real
docker compose logs -f backend

# Reiniciar apenas o backend após rebuild
docker compose up -d --build backend

# Acessar o banco de dados
docker compose exec postgres psql -U proxlab -d proxlab

# Parar tudo (mantém volumes)
docker compose down

# Parar e remover volumes (reseta o banco)
docker compose down -v
```

---

## Arquitetura

```
Navegador (porta 80)
    └── nginx (frontend container)
         ├── /            → arquivos estáticos Next.js
         └── /api/*       → proxy → backend:8080
                                        └── PostgreSQL (porta 5432, interno)
                                        └── Proxmox VE (rede da faculdade)
```

O backend acessa o Proxmox diretamente pela rede da faculdade — a rede Docker (`proxlab`) usa o driver `bridge` padrão, que tem acesso à rede do host por roteamento normal.
