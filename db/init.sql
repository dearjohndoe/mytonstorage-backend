
-- SCHEMAS

CREATE SCHEMA IF NOT EXISTS providers AUTHORIZATION pguser;

CREATE SCHEMA IF NOT EXISTS files AUTHORIZATION pguser;

CREATE SCHEMA IF NOT EXISTS system AUTHORIZATION pguser;

CREATE SCHEMA IF NOT EXISTS public AUTHORIZATION pguser;

-- TABLES

CREATE TABLE IF NOT EXISTS system.params
(
    key character varying(256) COLLATE pg_catalog."default" NOT NULL,
    value character varying(1024) COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone,
    CONSTRAINT params_pkey PRIMARY KEY (key)
);

INSERT INTO system.params (key, value) VALUES ('max_files_count', (5000)::text)
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS providers.notifications
(
    provider_pubkey character varying(64) COLLATE pg_catalog."default" NOT NULL,
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    storage_contract character varying(64) COLLATE pg_catalog."default" NOT NULL,
    size bigint NOT NULL,
    notify_attempts smallint NOT NULL DEFAULT 0,
    updated_at timestamp with time zone DEFAULT now(),
    download_checks smallint NOT NULL DEFAULT 0,
    downloaded bigint NOT NULL DEFAULT 0,
    notified boolean NOT NULL DEFAULT false,
    CONSTRAINT notifications_pkey PRIMARY KEY (provider_pubkey, storage_contract)
);

CREATE TABLE IF NOT EXISTS providers.notifications_history
(
    provider_pubkey character varying(64) COLLATE pg_catalog."default" NOT NULL,
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    storage_contract character varying(64) COLLATE pg_catalog."default" NOT NULL,
    size bigint NOT NULL,
    notify_attempts smallint NOT NULL DEFAULT 0,
    archived_at timestamp without time zone NOT NULL DEFAULT now(),
    download_checks integer DEFAULT 0,
    downloaded bigint DEFAULT 0,
    CONSTRAINT notifications_history_pkey PRIMARY KEY (provider_pubkey, storage_contract)
);

CREATE TABLE IF NOT EXISTS files.bag_users
(
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    user_address character varying(64) COLLATE pg_catalog."default" NOT NULL,
    storage_contract character varying(64) COLLATE pg_catalog."default",
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    notify_attempts smallint NOT NULL DEFAULT 0,
    CONSTRAINT bag_users_pkey PRIMARY KEY (bagid, user_address)
);

CREATE TABLE IF NOT EXISTS files.bag_users_history
(
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    user_address character varying(64) COLLATE pg_catalog."default" NOT NULL,
    storage_contract character varying(64) COLLATE pg_catalog."default",
    deleted_at timestamp with time zone DEFAULT now(),
    notify_attempts smallint NOT NULL DEFAULT 0,
    CONSTRAINT bag_users_history_pkey PRIMARY KEY (bagid, user_address)
);

CREATE TABLE IF NOT EXISTS files.bags
(
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    description text COLLATE pg_catalog."default" NOT NULL DEFAULT ''::text,
    size bigint NOT NULL DEFAULT 0,
    created_at timestamp with time zone DEFAULT now(),
    files_size bigint NOT NULL DEFAULT 0,
    CONSTRAINT bag_pkey PRIMARY KEY (bagid)
);

CREATE TABLE IF NOT EXISTS files.blacklist
(
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    admin character varying(64) COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT blacklist_pkey PRIMARY KEY (bagid)
);

CREATE TABLE IF NOT EXISTS files.blacklist_history
(
    id integer NOT NULL DEFAULT nextval('files.blacklist_history_id_seq'::regclass),
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    admin character varying(64) COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    banned boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    archived_at timestamp with time zone DEFAULT now(),
    CONSTRAINT blacklist_history_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS files.reports
(
    id integer NOT NULL DEFAULT nextval('files.reports_id_seq'::regclass),
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    sender text COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT reports_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS files.reports_archive
(
    id integer NOT NULL DEFAULT nextval('files.reports_archive_id_seq'::regclass),
    bagid character varying(64) COLLATE pg_catalog."default" NOT NULL,
    admin text COLLATE pg_catalog."default" NOT NULL,
    sender text COLLATE pg_catalog."default" NOT NULL,
    reason text COLLATE pg_catalog."default" NOT NULL,
    comment text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp with time zone,
    accepted boolean NOT NULL DEFAULT false,
    CONSTRAINT reports_archive_pkey PRIMARY KEY (id),
    CONSTRAINT reports_archive_bagid_admin_key UNIQUE (bagid, admin)
);

-- TRIGGERS AND FUNCTIONS

CREATE FUNCTION files.log_blacklist_changes()
    RETURNS trigger
    LANGUAGE plpgsql
    COST 100
    VOLATILE NOT LEAKPROOF
AS $BODY$
BEGIN
    -- Handle DELETE
    IF TG_OP = 'DELETE' THEN
        INSERT INTO files.blacklist_history (
            bagid,
            admin,
            reason,
            comment,
            banned,
            created_at
        ) VALUES (
            OLD.bagid,
            OLD.admin,
            OLD.reason,
            OLD.comment,
            false,
            NOW()
        );
        RETURN OLD;
    END IF;

    -- Handle INSERT and UPDATE
    INSERT INTO files.blacklist_history (
        bagid,
        admin,
        reason,
        comment,
        banned,
        created_at
    ) VALUES (
        NEW.bagid,
        NEW.admin,
        NEW.reason,
        NEW.comment,
        true,
        COALESCE(NEW.created_at, NOW())
    );
    RETURN NEW;
END;
$BODY$;

CREATE FUNCTION files.bag_users_history_insert()
    RETURNS trigger
    LANGUAGE plpgsql
    COST 100
    VOLATILE NOT LEAKPROOF
AS $BODY$
BEGIN
    INSERT INTO files.bag_users_history (bagid, user_address, storage_contract, deleted_at, notify_attempts)
    VALUES (OLD.bagid, OLD.user_address, OLD.storage_contract, now(), OLD.notify_attempts)
    ON CONFLICT (bagid, user_address) DO NOTHING;
    RETURN OLD;
END;
$BODY$;

CREATE FUNCTION providers.notifications_history_insert()
    RETURNS trigger
    LANGUAGE plpgsql
    COST 100
    VOLATILE NOT LEAKPROOF
AS $BODY$
BEGIN
    INSERT INTO providers.notifications_history (provider_pubkey, bagid, storage_contract, size, notify_attempts, download_checks, downloaded, archived_at)
    VALUES (OLD.provider_pubkey, OLD.bagid, OLD.storage_contract, OLD.size, OLD.notify_attempts, OLD.download_checks, OLD.downloaded, now());
    RETURN OLD;
END;
$BODY$;

CREATE OR REPLACE TRIGGER trigger_blacklist_delete
    AFTER DELETE
    ON files.blacklist
    FOR EACH ROW
    EXECUTE FUNCTION files.log_blacklist_changes();

CREATE OR REPLACE TRIGGER trigger_blacklist_insert
    AFTER INSERT
    ON files.blacklist
    FOR EACH ROW
    EXECUTE FUNCTION files.log_blacklist_changes();

CREATE OR REPLACE TRIGGER trigger_blacklist_update
    AFTER UPDATE 
    ON files.blacklist
    FOR EACH ROW
    EXECUTE FUNCTION files.log_blacklist_changes();

CREATE OR REPLACE TRIGGER trg_bag_users_history_insert
    AFTER DELETE
    ON files.bag_users
    FOR EACH ROW
    EXECUTE FUNCTION files.bag_users_history_insert();

CREATE OR REPLACE TRIGGER notifications_history_trigger
    AFTER DELETE
    ON providers.notifications
    FOR EACH ROW
    EXECUTE FUNCTION providers.notifications_history_insert();



