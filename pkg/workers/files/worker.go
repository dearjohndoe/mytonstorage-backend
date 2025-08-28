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
	GetProvidersToNotify(ctx context.Context, limit int, notifyAttempts int) (notifications []db.ProviderNotification, err error)
	IncreaseNotifyAttempts(ctx context.Context, notifications []db.ProviderNotification) error
	ArchiveNotifications(ctx context.Context, notifications []db.ProviderNotification) error
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
	RemoveUnusedFiles(ctx context.Context) (interval time.Duration, err error)
	RemoveOldUnpaidFiles(ctx context.Context) (interval time.Duration, err error)
	TriggerProvidersDownload(ctx context.Context) (interval time.Duration, err error)
	CollectContractProvidersToNotify(ctx context.Context) (interval time.Duration, err error)
}

// This worker check table bags and if some bag have no users(in bag_users) it will be removed from db and from disk.
func (w *filesWorker) RemoveUnusedFiles(ctx context.Context) (interval time.Duration, err error) {
	const (
		failureInterval = 5 * time.Second
		successInterval = 1 * time.Minute
	)

	log := w.logger.With("worker", "RemoveUnusedFiles")

	interval = successInterval

	// TODO: first check if they paid
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

// Removes files that are unpaid and older than the choosen period
func (w *filesWorker) RemoveOldUnpaidFiles(ctx context.Context) (interval time.Duration, err error) {
	const (
		failureInterval = 5 * time.Second
		successInterval = 1 * time.Minute
	)

	log := w.logger.With("worker", "RemoveOldUnpaidFiles")

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

	// Defer increasing attempts if there's an error
	defer func() {
		if err != nil {
			_ = w.providersDb.IncreaseNotifyAttempts(ctx, providersToNotify)
		}
	}()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	var notified []db.ProviderNotification
	for _, provider := range providersToNotify {
		toProof := r.Uint64() % uint64(math.Max(float64(provider.Size), 1))
		sc, cErr := address.ParseAddr(provider.StorageContract)
		if cErr != nil {
			log.Error("failed to parse storage contract address",
				"error", cErr.Error(),
				"storage_contract", provider.StorageContract)
			continue
		}

		func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			providerKey, err := hex.DecodeString(provider.ProviderPubkey)
			if err != nil {
				log.Error("failed to decode provider pubkey",
					"error", err.Error(),
					"provider_pubkey", provider.ProviderPubkey)
				return
			}

			_, pErr := w.provider.RequestStorageInfo(timeoutCtx, providerKey, sc, toProof)
			if pErr != nil {
				log.Error("failed to notify provider",
					"error", pErr.Error(),
					"provider_pubkey", provider.ProviderPubkey)
				return
			}

			notified = append(notified, provider)
		}()
	}

	if len(notified) > 0 {
		aErr := w.providersDb.ArchiveNotifications(ctx, notified)
		if aErr != nil {
			err = fmt.Errorf("failed to archive notifications: %w", aErr)
			interval = failureInterval
			return
		}

		var iErr error
		if len(notified) < len(providersToNotify) {
			iErr = w.providersDb.IncreaseNotifyAttempts(ctx, providersToNotify)
			if iErr != nil {
				log.Error("failed to increase notify attempts", "error", iErr.Error())
			}
		}

		if iErr == nil {
			log.Info("Providers successfully notified", "count", len(notified))
		}
	}

	return
}

// Collects providers for each storage contract that needs to be notified
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
