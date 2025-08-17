package providers

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-storage-provider/pkg/contract"
	"github.com/xssnick/tonutils-storage-provider/pkg/transport"
	"github.com/xssnick/tonutils-storage/provider"

	tonstorage "mytonstorage-backend/pkg/clients/ton-storage"
	"mytonstorage-backend/pkg/models"
	v1 "mytonstorage-backend/pkg/models/api/v1"
	"mytonstorage-backend/pkg/utils"
)

const (
	providersLimit         = 256
	providerRequestTimeout = 5 * time.Second
)

type storage interface {
	GetBag(ctx context.Context, bagId string) (*tonstorage.BagDetailed, error)
}

type service struct {
	storage        storage
	provider       *transport.Client
	maxAllowedSpan uint64
	logger         *slog.Logger
}

type Providers interface {
	FetchProvidersRates(ctx context.Context, req v1.OffersRequest) (resp v1.ProviderRatesResponse, err error)
	InitStorageContract(ctx context.Context, info v1.InitStorageContractRequest, providers []v1.ProviderShort) (resp v1.Transaction, err error)

	fetchProviderRates(ctx context.Context, bagSize uint64, providerKey string) (offer *v1.ProviderOffer, reason string)
}

func (s *service) FetchProvidersRates(ctx context.Context, req v1.OffersRequest) (resp v1.ProviderRatesResponse, err error) {
	log := s.logger.With(
		"method", "FetchProvidersRates",
		"bag_id", req.BagID,
		"providers", req.Providers)

	if len(req.Providers) > providersLimit {
		log.Error("too many providers requested", slog.Int("limit", providersLimit))
		err = models.NewAppError(models.BadRequestErrorCode, "too many providers requested")
		return
	}

	if len(req.Providers) == 0 {
		return
	}

	details, err := s.storage.GetBag(ctx, req.BagID)
	if err != nil {
		log.Error("failed to get bag details", slog.String("error", err.Error()))
		err = models.NewAppError(models.ServiceUnavailableCode, "failed to get bag details")
		return
	}

	for _, provider := range req.Providers {
		func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, providerRequestTimeout)
			defer cancel()

			// todo: add retries
			rate, reason := s.fetchProviderRates(timeoutCtx, details.BagSize, provider)
			if reason != "" {
				resp.Declines = append(resp.Declines, v1.ProviderDecline{
					ProviderKey: provider,
					Reason:      reason,
				})

				return
			}

			resp.Offers = append(resp.Offers, *rate)
		}()
	}

	return resp, nil
}

func (s *service) InitStorageContract(ctx context.Context, info v1.InitStorageContractRequest, providers []v1.ProviderShort) (resp v1.Transaction, err error) {
	log := s.logger.With(
		"method", "InitStorageContract",
		"bag_id", info.BagID,
		"owner", info.OwnerAddress,
		"amount", info.Amount)

	if len(providers) > providersLimit {
		log.Error("too many providers requested", slog.Int("limit", providersLimit))
		err = models.NewAppError(models.BadRequestErrorCode, "too many providers requested")
		return
	}

	if len(providers) == 0 {
		return
	}

	details, err := s.storage.GetBag(ctx, info.BagID)
	if err != nil {
		log.Error("failed to get bag details", slog.String("error", err.Error()))
		err = models.NewAppError(models.ServiceUnavailableCode, "failed to get bag details")
		return
	}

	merkle, err := hex.DecodeString(details.MerkleHash)
	if err != nil {
		log.Error("failed to decode merkle hash", slog.String("error", err.Error()))
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	torrentHash, err := hex.DecodeString(info.BagID)
	if err != nil {
		log.Error("failed to decode torrent hash", slog.String("error", err.Error()))
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	ownerAddr, err := address.ParseAddr(info.OwnerAddress)
	if err != nil {
		log.Error("failed to parse owner address", slog.String("error", err.Error()))
		err = models.NewAppError(models.BadRequestErrorCode, "invalid owner address")
		return
	}

	addr, sx, _, err := contract.PrepareV1DeployData(torrentHash, merkle, details.BagSize, details.PieceSize, ownerAddr, nil)
	if err != nil {
		log.Error("failed to prepare contract deploy data", slog.String("error", err.Error()))
		err = models.NewAppError(models.ServiceUnavailableCode, "failed to prepare contract deploy data")
		return
	}

	fmt.Printf("code hash: %x\n", sx.Code.Hash())

	var prs []contract.ProviderV1
	for _, p := range providers {
		d, dErr := hex.DecodeString(p.Pubkey)
		if dErr != nil {
			log.Error("failed to decode provider address", slog.String("error", dErr.Error()))
			err = models.NewAppError(models.BadRequestErrorCode, "invalid provider address")
			return
		}

		pAddr := address.NewAddress(0, 0, d)
		if pAddr == nil {
			log.Error("failed to parse provider address", "provider", p.Pubkey)
			err = models.NewAppError(models.BadRequestErrorCode, "invalid provider address")
			return
		}

		prs = append(prs, contract.ProviderV1{
			Address:       pAddr,
			MaxSpan:       uint32(p.MaxSpan),
			PricePerMBDay: tlb.FromNanoTON(new(big.Int).SetUint64(p.PricePerMBDay)),
		})
	}

	_, stateInit, body, err := contract.PrepareV1DeployData(torrentHash, merkle, details.BagSize, details.PieceSize, ownerAddr, prs)
	if err != nil {
		log.Error("failed to prepare contract deploy data", slog.String("error", err.Error()))
		err = models.NewAppError(models.ServiceUnavailableCode, "failed to prepare contract deploy data")
		return
	}

	siCell, err := tlb.ToCell(stateInit)
	if err != nil {
		log.Error("failed to convert state init to cell", slog.String("error", err.Error()))
		err = models.NewAppError(models.ServiceUnavailableCode, "failed to parse state init")
		return
	}

	b := base64.StdEncoding.EncodeToString(body.ToBOC())
	si := base64.StdEncoding.EncodeToString(siCell.ToBOC())

	resp = v1.Transaction{
		Address:   addr.String(),
		Body:      b,
		StateInit: si,
		Amount:    info.Amount,
	}

	return
}

func (s *service) fetchProviderRates(ctx context.Context, bagSize uint64, providerKey string) (offer *v1.ProviderOffer, reason string) {
	log := s.logger.With(
		"method", "fetchProviderRates",
		"bag_size", bagSize,
		"provider_key", providerKey)

	pk, err := utils.ToHashBytes(providerKey)
	if err != nil {
		log.Error("failed to parse provider hash", slog.String("error", err.Error()))
		reason = "invalid pubkey"
		return
	}

	rates, err := s.provider.GetStorageRates(ctx, pk, bagSize)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Error("provider rates request timed out", slog.String("error", err.Error()))
			reason = "long response time"
			return
		}

		log.Error("failed to fetch rates", slog.String("error", err.Error()))
		reason = "can't fetch rates"
		return
	}

	if rates.SpaceAvailableMB < bagSize {
		reason = "not enough space"
		return
	}

	if uint64(rates.MaxSpan) > s.maxAllowedSpan {
		rates.MaxSpan = uint32(s.maxAllowedSpan)
	}

	if rates.MinSpan > rates.MaxSpan {
		rates.MinSpan = rates.MaxSpan
	}

	if !rates.Available {
		reason = "not available"
		return
	}

	p := provider.ProviderRates{
		Available:        rates.Available,
		RatePerMBDay:     tlb.FromNanoTON(new(big.Int).SetBytes(rates.RatePerMBDay)),
		MinBounty:        tlb.FromNanoTON(new(big.Int).SetBytes(rates.MinBounty)),
		SpaceAvailableMB: rates.SpaceAvailableMB,
		MinSpan:          rates.MinSpan,
		MaxSpan:          rates.MaxSpan,

		Size: bagSize,
	}

	o := provider.CalculateBestProviderOffer(&p)

	offer = &v1.ProviderOffer{
		OfferSpan:     uint64(o.Span),
		PricePerDay:   o.PerDayNano.Uint64(),
		PricePerProof: o.PerProofNano.Uint64(),
		PricePerMB:    o.RatePerMBNano.Uint64(),

		Provider: v1.ProviderContractData{
			Key:          strings.ToUpper(providerKey),
			MinBounty:    tlb.FromNanoTON(new(big.Int).SetBytes(rates.MinBounty)).String(),
			MinSpan:      uint64(rates.MinSpan),
			MaxSpan:      uint64(rates.MaxSpan),
			RatePerMBDay: new(big.Int).SetBytes(rates.RatePerMBDay).Uint64(),
		},
	}

	return
}

func NewService(provider *transport.Client, storage storage, maxAllowedSpanDays uint32, logger *slog.Logger) Providers {
	return &service{
		provider:       provider,
		maxAllowedSpan: uint64(maxAllowedSpanDays) * 24 * 60 * 60,
		storage:        storage,
		logger:         logger,
	}
}
