package tonclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	pContract "github.com/xssnick/tonutils-storage-provider/pkg/contract"

	"mytonstorage-backend/pkg/utils"
)

const (
	getProvidersRetries = 5
	retries             = 20
	singleQueryTimeout  = 5 * time.Second
)

type client struct {
	clientPool *liteclient.ConnectionPool
	logger     *slog.Logger
}

type Client interface {
	GetProvidersInfo(ctx context.Context, addrs []string) (contractsProviders []StorageContractProviders, err error)
}

func (c *client) GetProvidersInfo(ctx context.Context, addrs []string) (contractsProviders []StorageContractProviders, err error) {
	log := c.logger.With("method", "GetProvidersInfo")
	api := ton.NewAPIClient(c.clientPool).WithTimeout(singleQueryTimeout).WithRetry(retries)
	block, err := api.GetMasterchainInfo(ctx)
	if err != nil {
		err = fmt.Errorf("get masterchain info err: %w", err)
		return
	}

	contractsProviders = make([]StorageContractProviders, 0, len(addrs))
	for _, a := range addrs {
		addr, err := address.ParseAddr(a)
		if err != nil {
			log.Error("invalid address", slog.String("address", a), slog.String("error", err.Error()))
			continue
		}

		var info []pContract.ProviderDataV1
		var coins tlb.Coins
		err = utils.TryNTimes(func() error {
			var cErr error
			info, coins, cErr = pContract.GetProvidersV1(ctx, api, block, addr)
			return cErr
		}, getProvidersRetries)
		if err != nil {
			log.Error("get providers info", slog.String("address", a), slog.String("error", err.Error()))
			continue
		}

		providers := make([]Provider, 0, len(info))
		for _, p := range info {
			providers = append(providers, Provider{
				Key:           string(p.Key),
				LastProofTime: p.LastProofAt,
				RatePerMBDay:  p.RatePerMB.Nano().Uint64(),
				MaxSpan:       p.MaxSpan,
			})
		}

		if len(providers) == 0 {
			continue
		}

		contractsProviders = append(contractsProviders, StorageContractProviders{
			Address:   a,
			Balance:   coins.Nano().Uint64(),
			Providers: providers,
		})
	}

	return
}

func NewClient(ctx context.Context, configUrl string, logger *slog.Logger) (Client, error) {
	clientPool := liteclient.NewConnectionPool()

	err := clientPool.AddConnectionsFromConfigUrl(ctx, configUrl)
	if err != nil {
		panic(err)
	}

	return &client{
		clientPool: clientPool,
		logger:     logger,
	}, nil
}
