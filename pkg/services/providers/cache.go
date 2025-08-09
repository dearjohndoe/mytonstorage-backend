package providers

import (
	"context"
	"fmt"
	"time"

	"mytonstorage-backend/pkg/cache"
	v1 "mytonstorage-backend/pkg/models/api/v1"
)

type providersCache struct {
	cache *cache.SimpleCache
	svc   Providers
}

func (c *providersCache) FetchProvidersRates(ctx context.Context, req v1.OffersRequest) (resp v1.ProviderRatesResponse, err error) {
	// cached inner method fetchProviderRates below, so we good here
	return c.svc.FetchProvidersRates(ctx, req)
}

func (c *providersCache) InitStorageContract(ctx context.Context, info v1.InitStorageContractRequest, providers []v1.ProviderShort) (resp v1.Transaction, err error) {
	return c.svc.InitStorageContract(ctx, info, providers)
}

func (c *providersCache) fetchProviderRates(ctx context.Context, bagSize uint64, providerKey string) (offer *v1.ProviderOffer, reason string) {
	key := fmt.Sprintf("pr_%s_%d", providerKey, bagSize)
	if cached, ok := c.cache.Get(key); ok {
		if offer, ok = cached.(*v1.ProviderOffer); ok {
			return
		}
	}

	offer, reason = c.svc.fetchProviderRates(ctx, bagSize, providerKey)
	if offer != nil && reason == "" {
		c.cache.Set(key, offer)
	}

	return
}

func NewProvidersCache(svc Providers) Providers {
	return &providersCache{
		cache: cache.NewSimpleCache(1 * time.Hour),
		svc:   svc,
	}
}
