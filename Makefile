.PHONY: run build test migrate

# Sobe todos os serviços (postgres, backend, frontend) em background.
# Na primeira vez faz o build das imagens automaticamente.
run:
	docker compose up -d --build

# Para todos os serviços.
stop:
	docker compose down

# Compila o binário do backend localmente (sem Docker).
build:
	cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/proxlab ./cmd/server

# Roda os testes unitários do backend (sem banco, sem Proxmox).
test:
	cd backend && go test ./internal/usecase/... -v -count=1

# Aplica todas as migrations SQL em ordem no banco que está rodando no Docker.
# Requer que o serviço postgres esteja saudável: make run antes de make migrate.
migrate:
	@for f in backend/migrations/*.sql; do \
		echo "→ $$f"; \
		docker compose exec -T postgres psql \
			-U $${DB_USER:-proxlab} \
			-d $${DB_NAME:-proxlab} \
			< "$$f"; \
	done
