CREATE TABLE IF NOT EXISTS containers (
    id            UUID         PRIMARY KEY,
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          VARCHAR(40)  NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    image         TEXT         NOT NULL,
    cpu_millis    INTEGER      NOT NULL,
    memory_mb     INTEGER      NOT NULL,
    port          INTEGER      NOT NULL DEFAULT 0,
    protocol      VARCHAR(4)   NOT NULL DEFAULT 'http',
    node_port     INTEGER      NOT NULL DEFAULT 0,
    env           JSONB        NOT NULL DEFAULT '{}',
    command       TEXT         NOT NULL DEFAULT '',
    persistent_gb INTEGER      NOT NULL DEFAULT 0,
    mount_path    TEXT         NOT NULL DEFAULT '',
    namespace     VARCHAR(10)  NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_containers_user_id ON containers(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_containers_user_name ON containers(user_id, name);
-- alocação atômica de NodePort: só uma linha por porta > 0
CREATE UNIQUE INDEX IF NOT EXISTS ux_containers_node_port ON containers(node_port) WHERE node_port > 0;
