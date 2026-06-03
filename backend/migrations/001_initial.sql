BEGIN;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tipos enumerados (idempotente)
DO $$ BEGIN
  CREATE TYPE user_role AS ENUM ('aluno', 'pesquisador');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE vm_status AS ENUM ('creating', 'running', 'stopped', 'error');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Tabela de usuários
CREATE TABLE IF NOT EXISTS users (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          user_role    NOT NULL DEFAULT 'aluno',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Tabela de máquinas virtuais
CREATE TABLE IF NOT EXISTS vms (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    proxmox_vm_id INTEGER      NOT NULL,
    name          VARCHAR(255) NOT NULL,
    cpu_cores     INTEGER      NOT NULL,
    memory_mb     INTEGER      NOT NULL,
    disk_gb       INTEGER      NOT NULL,
    os_template   VARCHAR(255) NOT NULL DEFAULT '',
    node          VARCHAR(100) NOT NULL,
    status        vm_status    NOT NULL DEFAULT 'creating',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vms_user_id ON vms(user_id);

COMMIT;
