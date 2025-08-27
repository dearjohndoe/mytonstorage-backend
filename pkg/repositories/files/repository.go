package files

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mytonstorage-backend/pkg/models/db"
)

type repository struct {
	db *pgxpool.Pool
}

type Repository interface {
	AddBag(ctx context.Context, bag db.BagInfo, userAddr string) error
	RemoveUserBagRelation(ctx context.Context, bagID, userAddress string) (int64, error)
	RemoveUnusedBags(ctx context.Context) (removed []string, err error)
	GetUnpaidBags(ctx context.Context, userID string) ([]db.UserBagInfo, error)
	MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (int64, error)

	GetBagsInfoShort(ctx context.Context, bagIDs []string) ([]db.BagDescription, error)

	GetNotifyInfo(ctx context.Context, limit int, notifyAttempts int) ([]db.BagStorageContract, error)
	IncreaseAttempts(ctx context.Context, bags []db.BagStorageContract) error
}

func (r *repository) AddBag(ctx context.Context, bag db.BagInfo, userAddr string) error {
	query := `
		WITH add_file AS (
			INSERT INTO files.bags (bagid, description, size, created_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (bagid) DO NOTHING
			RETURNING bagid
		)
		INSERT INTO files.bag_users (bagid, user_address, storage_contract, created_at, updated_at)
		VALUES ($1, $4, NULL, NOW(), NOW())
		ON CONFLICT (bagid, user_address) DO UPDATE 
			SET updated_at = NOW();
	`
	_, err := r.db.Exec(ctx, query, bag.BagID, bag.Description, bag.Size, userAddr)
	return err
}

func (r *repository) RemoveUnusedBags(ctx context.Context) (removed []string, err error) {
	query := `
		WITH to_remove AS (
			SELECT b.bagid
			FROM files.bags b
				LEFT JOIN files.bag_users bu ON b.bagid = bu.bagid
			WHERE bu.bagid IS NULL
		),
		remove AS (
			DELETE FROM files.bags
			WHERE bagid IN (SELECT bagid FROM to_remove)
			RETURNING bagid
		)
		SELECT bagid FROM remove;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var bagID string
		if err := rows.Scan(&bagID); err != nil {
			return nil, err
		}
		removed = append(removed, bagID)
	}

	return removed, nil
}

func (r *repository) RemoveUserBagRelation(ctx context.Context, bagID, userAddress string) (cnt int64, err error) {
	query := `
		DELETE FROM files.bag_users
		WHERE bagid = $1 AND user_address = $2;
	`
	row, err := r.db.Exec(ctx, query, bagID, userAddress)
	if err != nil {
		return
	}

	cnt = row.RowsAffected()

	return
}

func (r *repository) GetUnpaidBags(ctx context.Context, userID string) ([]db.UserBagInfo, error) {
	var bags []db.UserBagInfo
	query := `
		SELECT bagid, user_address, created_at, updated_at
		FROM files.bag_users
		WHERE user_address = $1 AND storage_contract IS NULL;
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var bag db.UserBagInfo
		var createdAt *time.Time
		var updatedAt *time.Time
		if err := rows.Scan(&bag.BagID, &bag.UserAddress, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		bag.CreatedAt = createdAt.Unix()
		bag.UpdatedAt = updatedAt.Unix()
		bags = append(bags, bag)
	}

	return bags, nil
}

func (r *repository) MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (cnt int64, err error) {
	query := `
		UPDATE files.bag_users
		SET storage_contract = $3
		WHERE bagid = $1 AND user_address = $2
		RETURNING 1;
	`
	row, err := r.db.Exec(ctx, query, bagID, userAddress, storageContract)
	if err != nil {
		return
	}

	cnt = row.RowsAffected()

	return
}

func (r *repository) GetBagsInfoShort(ctx context.Context, contracts []string) (descriptions []db.BagDescription, err error) {
	query := `
		SELECT bu.storage_contract, b.bagid, b.description, b.size
		FROM files.bag_users bu 
			JOIN files.bags b ON b.bagid = bu.bagid
		WHERE bu.storage_contract = ANY($1::text[])
	`

	rows, err := r.db.Query(ctx, query, contracts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var desc db.BagDescription
		if err := rows.Scan(&desc.ContractAddress, &desc.BagID, &desc.Description, &desc.Size); err != nil {
			return nil, err
		}
		descriptions = append(descriptions, desc)
	}

	return descriptions, nil
}

func (r *repository) GetNotifyInfo(ctx context.Context, limit int, notifyAttempts int) (resp []db.BagStorageContract, err error) {
	var info db.BagStorageContract
	query := `
		SELECT bu.bagid, bu.storage_contract, b.size
		FROM files.bag_users bu
			JOIN files.bags b ON b.bagid=bu.bagid
		WHERE bu.storage_contract IS NOT NULL 
			AND bu.notify_attempts >= 0 
			AND bu.notify_attempts < $2
		LIMIT $1;
	`
	rows, err := r.db.Query(ctx, query, limit, notifyAttempts)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		if sErr := rows.Scan(&info.BagID, &info.StorageContract, &info.Size); sErr != nil {
			err = sErr
			return
		}
		resp = append(resp, info)
	}
	return resp, nil
}

func (r *repository) IncreaseAttempts(ctx context.Context, bags []db.BagStorageContract) (err error) {
	query := `
		WITH cte AS (
			SELECT x.bagid, x.storage_contract
			FROM jsonb_array_elements($1::jsonb) AS x(bagid, storage_contract)
		)
		UPDATE files.bag_users
		SET notify_attempts = notify_attempts + 1
		WHERE (bagid, storage_contract) IN (SELECT bagid, storage_contract FROM cte)
	`
	_, err = r.db.Exec(ctx, query, bags)
	return
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{
		db: db,
	}
}
