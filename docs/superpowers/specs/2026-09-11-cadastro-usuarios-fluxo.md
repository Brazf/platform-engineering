# Cadastro de usuários — fluxo por convite (spec)

**Data:** 2026-09-11 · **Status:** parte 1 implementada; parte 2 pendente

## Decisão

Não existe mais auto-cadastro. Só usuários com perfil `pesquisador` criam contas, para alunos (e outros
pesquisadores) do seu grupo. O aluno conclui o próprio cadastro na tela de **primeiro acesso**, com um código
recebido por e-mail.

## Fluxo alvo

1. Pesquisador logado abre **Cadastrar novo usuário** (card na página inicial, visível só para `pesquisador`).
2. Preenche e-mail e perfil. Backend cria o usuário com status `pendente`, gera um **código de ativação** (ex.: 6
   dígitos ou token curto, com validade) e envia por e-mail para o endereço cadastrado.
3. Aluno abre `/primeiro-acesso`, informa **e-mail + código**, e o sistema pede que ele **crie a própria senha**.
4. Conta vira `ativa`; login normal a partir daí. Códigos expirados podem ser reenviados pelo pesquisador.

## Parte 1 — feita em 2026-09-11

- `POST /api/auth/register` (público) foi substituído por **`POST /api/users`**, protegido por JWT +
  `RequireRole("pesquisador")`. Payload continua `{ email, password, role }`.
- Página inicial mostra o card **Cadastrar novo usuário** só para `pesquisador`.
- `/cadastrar-usuario` virou formulário real: e-mail, perfil, senha provisória + confirmação, tela de resumo
  com "Cadastrar outro". A senha provisória é temporária até a parte 2 existir.
- Bootstrap: o **primeiro pesquisador** é criado direto no banco (SQL/seed), já que não há rota pública.

## Parte 2 — pendente

Backend
- Entidade `User`: campos `status` (`pendente` | `ativo`), `activation_code_hash`, `activation_expires_at`.
- `POST /api/users` deixa de receber `password`; gera código, salva hash, envia e-mail (SMTP via env
  `SMTP_HOST/PORT/USER/PASSWORD/FROM`), responde 201 sem o código.
- `POST /api/auth/first-access` (público): `{ email, code, password }` → valida código e validade, define senha,
  marca `ativo`. Erros genéricos (não revelar se o e-mail existe).
- `POST /api/users/:id/resend-code` (pesquisador): novo código e novo e-mail.
- Login recusa usuário `pendente` com mensagem orientando o primeiro acesso.
- Migration `004_users_activation.sql`.

Frontend
- `/primeiro-acesso` (mock em `(acesso)/primeiro-acesso`) vira tela real: e-mail + código → nova senha.
- `/cadastro` (auto-cadastro público) é removido; link na tela de login aponta para primeiro acesso.
- `/cadastrar-usuario` perde os campos de senha e mostra "código enviado para <e-mail>".
- Listagem de usuários cadastrados pelo pesquisador com status e botão "Reenviar código" (opcional).

## Fora de escopo

Nome e CPF do mock antigo (não existem na entidade); recuperação de senha (telas `esqueceu-senha`,
`codigo-recuperacao`, `nova-senha` continuam mock).

## Parte 3 — Perfil do usuário (concluída em 2026-09-25)

Tela **Meu perfil**, acessível a qualquer usuário logado (link no cabeçalho ou card na página inicial).
- Campos editáveis: **nome** (novo campo em `User`, `name`, opcional até então) e **senha** (exige a senha atual +
  nova senha + confirmação).
- **E-mail não pode ser alterado** (é o login; exibido só para leitura).
- Backend: `GET /api/me` (dados do próprio usuário), `PATCH /api/me` (`{ name }`),
  `PUT /api/me/password` (`{ current_password, new_password }`; 401 se a senha atual não confere).
- Migration: coluna `name` em `users`.
- Desvio (2026-09-25): senha atual errada responde **403**, não 401 — o front trata todo 401 como sessão expirada e deslogaria o usuário. Acesso pelo ícone de perfil no header (entre notificações e Sair).

## Parte 4 — Usuários cadastrados (pendente, 2026-09-12)

Card **Usuários cadastrados** na página inicial, visível só para `pesquisador`, levando a `/usuarios`.
- Lista **todos os usuários** (alunos e pesquisadores) com nome, e-mail, perfil, status (pendente/ativo) e data.
- Ações permitidas ao pesquisador nessa tela: **alterar o e-mail** de um usuário e **excluir** o usuário. Nada
  mais é editável ali (perfil e senha não).
- Regras: não pode excluir a si mesmo. **Excluir um usuário apaga todos os recursos dele** (decisão de
  2026-09-11): o backend deprovisiona cada VM (Terraform destroy; VMs sem workspace só saem do banco) e cada
  container (cluster + registro) antes de remover o usuário. Se algum recurso falhar, a exclusão para e
  responde 503 dizendo o que ficou; o front pede confirmação listando a quantidade de VMs e containers que
  serão apagados.
- Backend: `GET /api/users`, `PATCH /api/users/:id` (`{ email }`, 409 se já em uso), `DELETE /api/users/:id`;
  todos com JWT + `RequireRole("pesquisador")`.
- Front: lista no padrão `ResourceCard`/`Field`, edição de e-mail inline ou em modal pequeno, exclusão com
  confirmação. Segue o mesmo `form-ui`.

## Ordem sugerida para 2026-09-12

1. Parte 2 (código por e-mail + primeiro acesso) — depende de SMTP.
2. Parte 4 (lista de usuários) — pode andar em paralelo, não depende de e-mail.
3. Parte 3 (perfil) — precisa da coluna `name`; fazer a migration junto com a parte 2.
