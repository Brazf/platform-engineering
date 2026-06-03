CREATE TYPE vm_status AS ENUM ('creating', 'running', 'stopped', 'error');

CREATE TABLE IF NOT EXISTS vms (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    proxmox_vm_id INTEGER     NOT NULL,
    name          VARCHAR(255) NOT NULL,
    cpu           INTEGER     NOT NULL,
    memory        INTEGER     NOT NULL,
    disk          INTEGER     NOT NULL,
    status        vm_status   NOT NULL DEFAULT 'creating',
    node          VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vms_user_id ON vms(user_id);
