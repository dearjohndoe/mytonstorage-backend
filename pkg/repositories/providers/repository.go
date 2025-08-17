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
	GetProvidersToNotify(ctx context.Context, limit int, notifyAttempts int) (notifications []db.ProviderNotification, err error)
	IncreaseNotifyAttempts(ctx context.Context, notifications []db.ProviderNotification) error
	ArchiveNotifications(ctx context.Context, notifications []db.ProviderNotification) error
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

func (r *repository) GetProvidersToNotify(ctx context.Context, limit int, notifyAttempts int) (notifications []db.ProviderNotification, err error) {
	query := `
		SELECT bagid, storage_contract, provider_pubkey, size
		FROM providers.notifications
		WHERE attempts < $2
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

func (r *repository) IncreaseNotifyAttempts(ctx context.Context, notifications []db.ProviderNotification) error {
	query := `
		UPDATE providers.notifications
		SET notify_attempts = attempts + 1
		WHERE (storage_contract, provider_pubkey) IN (
			SELECT storage_contract, provider_pubkey
			FROM jsonb_to_recordset($1::jsonb) AS x(storage_contract text, provider_pubkey text)
		)
	`
	_, err := r.db.Exec(ctx, query, notifications)
	return err
}

func (r *repository) ArchiveNotifications(ctx context.Context, notifications []db.ProviderNotification) (err error) {
	query := `
        WITH cte AS (
            SELECT storage_contract, provider_pubkey
            FROM jsonb_to_recordset($1::jsonb) AS x(storage_contract text, provider_pubkey text)
        ), archive AS (
            INSERT INTO providers.notifications_history (storage_contract, provider_pubkey, bagid, size, attempts)
            SELECT storage_contract, provider_pubkey, bagid, size, attempts
            FROM providers.notifications
            WHERE (storage_contract, provider_pubkey) IN (SELECT storage_contract, provider_pubkey FROM cte)
        )
        DELETE FROM providers.notifications
        WHERE (storage_contract, provider_pubkey) IN (SELECT storage_contract, provider_pubkey FROM cte)
    `
	_, err = r.db.Exec(ctx, query, notifications)
	return
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{
		db: db,
	}
}
