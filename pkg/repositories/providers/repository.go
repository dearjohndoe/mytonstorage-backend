package providers

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"mytonstorage-backend/pkg/models/db"
)

type repository struct {
	db *pgxpool.Pool
}

type Repository interface {
	AddProviderToNotifyQueue(ctx context.Context, notifications []db.ProviderNotification) error
	GetProvidersInProgress(ctx context.Context, limit int, maxDownloadChecks int) (notifications []db.ProviderNotification, err error)
	GetProvidersToNotify(ctx context.Context, limit int, notifyAttempts int) (notifications []db.ProviderNotification, err error)
	IncreaseDownloadChecks(ctx context.Context, notifications []db.ProviderNotification) error
	IncreaseNotifyAttempts(ctx context.Context, notifications []db.ProviderNotification) error
	MarkAsNotified(ctx context.Context, notifications []db.ProviderNotification) error
}

func (r *repository) AddProviderToNotifyQueue(ctx context.Context, notifications []db.ProviderNotification) (err error) {
	query := `
        WITH cte AS (
            SELECT x.bagid, x.storage_contract, x.provider_pubkey, x.size
            FROM jsonb_to_recordset($1::jsonb) AS x(bagid text, storage_contract text, provider_pubkey text, size bigint)
        ), update_files AS (
            UPDATE files.bag_users
            SET notify_attempts = -1
            WHERE (bagid, storage_contract) IN (SELECT DISTINCT bagid, storage_contract FROM cte)
        )
        INSERT INTO providers.notifications (bagid, storage_contract, provider_pubkey, size)
        SELECT bagid, storage_contract, provider_pubkey, size
        FROM cte
        ON CONFLICT (provider_pubkey, storage_contract) DO NOTHING;
    `
	_, err = r.db.Exec(ctx, query, notifications)
	return
}

func (r *repository) GetProvidersInProgress(ctx context.Context, limit int, maxDownloadChecks int) (notifications []db.ProviderNotification, err error) {
	query := `
		SELECT bagid, storage_contract, provider_pubkey, size, downloaded
		FROM providers.notifications
		WHERE size > downloaded 
			AND notified
			AND download_checks <= $2
			AND updated_at < now() - interval '5 minute'
		ORDER BY updated_at ASC		-- oldest first
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, query, limit, maxDownloadChecks)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var provider db.ProviderNotification
		if err = rows.Scan(&provider.BagID, &provider.StorageContract, &provider.ProviderPubkey, &provider.Size, &provider.Downloaded); err != nil {
			return
		}
		notifications = append(notifications, provider)
	}
	return
}

func (r *repository) GetProvidersToNotify(ctx context.Context, limit int, notifyAttempts int) (notifications []db.ProviderNotification, err error) {
	query := `
		SELECT bagid, storage_contract, provider_pubkey, size
		FROM providers.notifications
		WHERE notify_attempts <= $2 AND NOT notified
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, query, limit, notifyAttempts)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var provider db.ProviderNotification
		if err = rows.Scan(&provider.BagID, &provider.StorageContract, &provider.ProviderPubkey, &provider.Size); err != nil {
			return
		}
		notifications = append(notifications, provider)
	}
	return
}

func (r *repository) IncreaseDownloadChecks(ctx context.Context, notifications []db.ProviderNotification) error {
	query := `
		WITH cte AS (
			SELECT storage_contract, provider_pubkey, downloaded
			FROM jsonb_to_recordset($1::jsonb) AS x(storage_contract text, provider_pubkey text, downloaded bigint)
		), upd AS (
			UPDATE providers.notifications n
			SET download_checks = download_checks + 1,
				downloaded = c.downloaded,
				updated_at = now()
			FROM cte c
			WHERE (n.storage_contract, n.provider_pubkey) = (c.storage_contract, c.provider_pubkey)
			RETURNING n.size, c.downloaded, c.provider_pubkey, c.storage_contract
		)
	`
	_, err := r.db.Exec(ctx, query, notifications)
	return err
}

func (r *repository) IncreaseNotifyAttempts(ctx context.Context, notifications []db.ProviderNotification) error {
	query := `
		UPDATE providers.notifications
		SET notify_attempts = notify_attempts + 1,
			updated_at = now()
		WHERE (storage_contract, provider_pubkey) IN (
			SELECT storage_contract, provider_pubkey
			FROM jsonb_to_recordset($1::jsonb) AS x(storage_contract text, provider_pubkey text)
		)
	`
	_, err := r.db.Exec(ctx, query, notifications)
	return err
}

func (r *repository) MarkAsNotified(ctx context.Context, notifications []db.ProviderNotification) error {
	query := `
		UPDATE providers.notifications
		SET notify_attempts = notify_attempts + 1,
			notified = true,
			updated_at = now()
		WHERE (storage_contract, provider_pubkey) IN (
			SELECT storage_contract, provider_pubkey
			FROM jsonb_to_recordset($1::jsonb) AS x(storage_contract text, provider_pubkey text)
		)
	`
	_, err := r.db.Exec(ctx, query, notifications)
	return err
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{
		db: db,
	}
}
