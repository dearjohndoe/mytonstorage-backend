package filesworker

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"slices"
	"strings"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-storage-provider/pkg/transport"

	tonclient "mytonstorage-backend/pkg/clients/ton"
	"mytonstorage-backend/pkg/models/db"
)

type filesDb interface {
	RemoveUnusedBags(ctx context.Context) (removed []string, err error)
	RemoveUnpaidBags(ctx context.Context, sec uint64) (bagids []string, err error)
	GetNotifyInfo(ctx context.Context, limit int, notifyAttempts int) (resp []db.BagStorageContract, err error)
	IncreaseAttempts(ctx context.Context, bags []db.BagStorageContract) error
}

type providersDb interface {
	AddProviderToNotifyQueue(ctx context.Context, notifications []db.ProviderNotification) error
	GetProvidersInProgress(ctx context.Context, limit int, maxDownloadChecks int) (notifications []db.ProviderNotification, err error)
	GetProvidersToNotify(ctx context.Context, limit int, notifyAttempts int) (notifications []db.ProviderNotification, err error)
	IncreaseDownloadChecks(ctx context.Context, notifications []db.ProviderNotification) error
	IncreaseNotifyAttempts(ctx context.Context, notifications []db.ProviderNotification) error
	MarkAsNotified(ctx context.Context, notifications []db.ProviderNotification) error
}

type storage interface {
	RemoveBag(ctx context.Context, bagId string, withFiles bool) error
}

type contractsClient interface {
	GetProvidersInfo(ctx context.Context, addrs []string) (contractsProviders []tonclient.StorageContractProviders, err error)
}

type filesWorker struct {
	filesDb             filesDb
	providersDb         providersDb
	tonstorage          storage
	provider            *transport.Client
	contractsClient     contractsClient
	unpaidFilesLifetime time.Duration
	logger              *slog.Logger
}

type Worker interface {
	RemoveUnpaidFiles(ctx context.Context) (interval time.Duration, err error)
	MarkToRemoveUnpaidFiles(ctx context.Context) (interval time.Duration, err error)

	TriggerProvidersDownload(ctx context.Context) (interval time.Duration, err error)
	DownloadChecker(ctx context.Context) (interval time.Duration, err error)

	CollectContractProvidersToNotify(ctx context.Context) (interval time.Duration, err error)
}

// This worker check table bags and if some bag have no users(in bag_users) it will be removed from db and from disk.
func (w *filesWorker) RemoveUnpaidFiles(ctx context.Context) (interval time.Duration, err error) {
	const (
		failureInterval = 5 * time.Second
		successInterval = 1 * time.Minute
	)

	log := w.logger.With("worker", "RemoveUnpaidFiles")

	interval = successInterval

	removed, err := w.filesDb.RemoveUnusedBags(ctx)
	if err != nil {
		interval = failureInterval
		return
	}

	for _, bagID := range removed {
		err = w.tonstorage.RemoveBag(ctx, bagID, true)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Info("Bag already deleted")
				continue
			}

			continue
		}
	}

	if len(removed) > 0 {
		log.Info("removed unused files", "count", len(removed))
	}

	return
}

func (w *filesWorker) MarkToRemoveUnpaidFiles(ctx context.Context) (interval time.Duration, err error) {
	const (
		failureInterval = 5 * time.Second
		successInterval = 1 * time.Minute
	)

	log := w.logger.With("worker", "MarkToRemoveUnpaidFiles")

	interval = successInterval

	removed, err := w.filesDb.RemoveUnpaidBags(ctx, uint64(w.unpaidFilesLifetime.Seconds()))
	if err != nil {
		interval = failureInterval
		return
	}

	if len(removed) > 0 {
		log.Info("removed old unpaid files", "count", len(removed))
	}

	return
}

func (w *filesWorker) TriggerProvidersDownload(ctx context.Context) (interval time.Duration, err error) {
	const (
		failureInterval         = 5 * time.Second
		successInterval         = 1 * time.Second
		nothingToUpdateInterval = 1 * time.Minute
		batch                   = 20
		maxNotifyAttempts       = 3
	)

	log := w.logger.With("worker", "TriggerProvidersDownload")

	interval = successInterval

	providersToNotify, err := w.providersDb.GetProvidersToNotify(ctx, batch, maxNotifyAttempts)
	if err != nil {
		err = fmt.Errorf("failed to get providers to notify: %w", err)
		interval = failureInterval
		return
	}

	if len(providersToNotify) < batch {
		interval = nothingToUpdateInterval
	}

	notified, failed := w.checkProvidersStorageInfo(ctx, providersToNotify)

	// Defer increasing attempts if there's an error
	defer func() {
		if err != nil {
			_ = w.providersDb.IncreaseNotifyAttempts(ctx, providersToNotify)
		}
	}()

	if len(failed) > 0 {
		_ = w.providersDb.IncreaseNotifyAttempts(ctx, failed)
		log.Warn("Some providers failed notification check", "failed_count", len(failed))
	}

	if len(notified) > 0 {
		aErr := w.providersDb.MarkAsNotified(ctx, notified)
		if aErr != nil {
			err = fmt.Errorf("failed to mark as notified: %w", aErr)
			interval = failureInterval
			return
		}

		log.Info("Providers successfully checked and marked as notified", "count", len(notified))
	}

	return
}

func (w *filesWorker) DownloadChecker(ctx context.Context) (interval time.Duration, err error) {
	const (
		failureInterval         = 5 * time.Second
		successInterval         = 1 * time.Second
		nothingToUpdateInterval = 1 * time.Minute
		batch                   = 20
		maxDownloadChecks       = 10
	)

	log := w.logger.With("worker", "DownloadChecker")

	interval = successInterval

	providersToCheck, err := w.providersDb.GetProvidersInProgress(ctx, batch, maxDownloadChecks)
	if err != nil {
		err = fmt.Errorf("failed to get providers to notify: %w", err)
		interval = failureInterval
		return
	}

	if len(providersToCheck) < batch {
		interval = nothingToUpdateInterval
	}

	checked, failed := w.checkProvidersStorageInfo(ctx, providersToCheck)

	if len(failed) > 0 {
		_ = w.providersDb.IncreaseDownloadChecks(ctx, failed)
		log.Info("Some providers failed download check", "failed_count", len(failed))
	}

	if len(checked) > 0 {
		aErr := w.providersDb.IncreaseDownloadChecks(ctx, checked)
		if aErr != nil {
			err = fmt.Errorf("failed to mark as notified: %w", aErr)
			interval = failureInterval
			return
		}

		log.Info("Providers successfully checked for download", "count", len(checked))
	}

	return
}

func (w *filesWorker) CollectContractProvidersToNotify(ctx context.Context) (interval time.Duration, err error) {
	const (
		failureInterval         = 5 * time.Second
		successInterval         = 1 * time.Second
		nothingToUpdateInterval = 1 * time.Minute
		batch                   = 10
		maxNotifyAttempts       = 3
	)

	log := w.logger.With("worker", "CollectContractProvidersToNotify")

	interval = successInterval

	// Get all bags that need to be downloaded by providers
	contractsToNotify, err := w.filesDb.GetNotifyInfo(ctx, batch, maxNotifyAttempts)
	if err != nil {
		err = fmt.Errorf("failed to get notify info: %w", err)
		interval = failureInterval
		return
	}

	if len(contractsToNotify) < batch {
		interval = nothingToUpdateInterval
	}

	var addrs []string
	for _, contract := range contractsToNotify {
		addrs = append(addrs, contract.StorageContract)
	}

	// Defer increasing attempts if there's an error
	defer func() {
		if err != nil {
			_ = w.filesDb.IncreaseAttempts(ctx, contractsToNotify)
		}
	}()

	// Get provider information for each storage contract
	contractsProviders, err := w.contractsClient.GetProvidersInfo(ctx, addrs)
	if err != nil {
		err = fmt.Errorf("failed to get providers info: %w", err)
		interval = failureInterval
		return
	}

	var providersToNotify []db.ProviderNotification
	for _, contract := range contractsProviders {
		sliceIndex := slices.IndexFunc(contractsToNotify, func(item db.BagStorageContract) bool {
			return item.StorageContract == contract.Address
		})
		if sliceIndex == -1 {
			continue
		}

		for _, provider := range contract.Providers {
			pk := hex.EncodeToString([]byte(provider.Key))

			providersToNotify = append(providersToNotify, db.ProviderNotification{
				BagID:           contractsToNotify[sliceIndex].BagID,
				StorageContract: contract.Address,
				ProviderPubkey:  pk,
				Size:            contractsToNotify[sliceIndex].Size,
			})
		}
	}

	if len(providersToNotify) == 0 {
		interval = nothingToUpdateInterval
		return
	}

	// Update the database with the new provider notifications + mark bags as notified
	err = w.providersDb.AddProviderToNotifyQueue(ctx, providersToNotify)
	if err != nil {
		err = fmt.Errorf("failed to add provider to notify queue: %w", err)
		interval = failureInterval
		return
	}

	if len(providersToNotify) > 0 {
		log.Info("Provider - contract relations added to notify queue", "count", len(providersToNotify))
	}

	return
}

func (w *filesWorker) checkProvidersStorageInfo(ctx context.Context, providers []db.ProviderNotification) (checked []db.ProviderNotification, failed []db.ProviderNotification) {
	log := w.logger.With("worker", "checkProvidersStorageInfo")

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, provider := range providers {
		toProof := r.Uint64() % uint64(math.Max(float64(provider.Size), 1))
		sc, cErr := address.ParseAddr(provider.StorageContract)
		if cErr != nil {
			log.Error("failed to parse storage contract address",
				"error", cErr.Error(),
				"storage_contract", provider.StorageContract)
			continue
		}

		fErr := func() error {
			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			providerKey, dErr := hex.DecodeString(provider.ProviderPubkey)
			if dErr != nil {
				log.Error("failed to decode provider pubkey",
					"error", dErr.Error(),
					"provider_pubkey", provider.ProviderPubkey)
				return dErr
			}

			info, rErr := w.provider.RequestStorageInfo(timeoutCtx, providerKey, sc, toProof)
			if rErr != nil {
				log.Error("failed to notify provider",
					"error", rErr.Error(),
					"provider_pubkey", provider.ProviderPubkey)
				return rErr
			}

			if info.Status == "error" {
				log.Error("provider returned error status",
					"error", info.Reason,
					"provider_pubkey", provider.ProviderPubkey)
				return fmt.Errorf("%s", info.Reason)
			}

			if len(info.Proof) > 0 {
				provider.Downloaded = info.Downloaded
				checked = append(checked, provider)
			}

			return nil
		}()
		if fErr != nil {
			failed = append(failed, provider)
		}
	}

	return
}

func NewWorker(
	filesDb filesDb,
	providersDb providersDb,
	tonstorage storage,
	provider *transport.Client,
	contractsClient contractsClient,
	unpaidFilesLifetime time.Duration,
	logger *slog.Logger,
) Worker {
	return &filesWorker{
		filesDb:             filesDb,
		providersDb:         providersDb,
		tonstorage:          tonstorage,
		provider:            provider,
		contractsClient:     contractsClient,
		unpaidFilesLifetime: unpaidFilesLifetime,
		logger:              logger,
	}
}
