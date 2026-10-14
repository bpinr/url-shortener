CREATE TABLE IF NOT EXISTS url (
	id         BIGSERIAL PRIMARY KEY,
	alias      TEXT        NOT NULL UNIQUE,
	url        TEXT        NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
