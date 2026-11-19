-- Resource mapping da GPU anexada à VM (vazio = sem GPU ou VM legada). Ver spec 2026-11-17.
ALTER TABLE vms ADD COLUMN IF NOT EXISTS gpu_mapping VARCHAR(64) NOT NULL DEFAULT '';
