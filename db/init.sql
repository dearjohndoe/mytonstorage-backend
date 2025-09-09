
-- SCHEMAS

CREATE SCHEMA providers AUTHORIZATION pguser;

CREATE SCHEMA files AUTHORIZATION pguser;

CREATE SCHEMA public AUTHORIZATION pg_database_owner;

CREATE SCHEMA system AUTHORIZATION pguser;

-- TABLES

CREATE TABLE IF NOT EXISTS system.params
(
    key character varying(256) COLLATE pg_catalog."default" NOT NULL,
    value character varying(1024) COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone,
    CONSTRAINT params_pkey PRIMARY KEY (key)
);

-- 4 GB
INSERT INTO system.params (key, value) VALUES ('max_files_size', (4::bigint << 30)::text)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system.params (key, value) VALUES ('max_files_count', (10000)::text)
ON CONFLICT (key) DO NOTHING;

