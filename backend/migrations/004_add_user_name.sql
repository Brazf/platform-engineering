-- Nome exibido em "Meu perfil" (opcional; e-mail continua sendo o login).
ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(120) NOT NULL DEFAULT '';
