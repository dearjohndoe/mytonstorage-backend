package tonstorage

import (
	"context"
	"time"

	"mytonstorage-backend/pkg/cache"
)

type cacheMiddleware struct {
	cache *cache.SimpleCache
	svc   Client
}

func (c *cacheMiddleware) Create(ctx context.Context, description, path string) (string, error) {
	return c.svc.Create(ctx, description, path)
}

func (c *cacheMiddleware) GetBag(ctx context.Context, bagId string) (*BagDetailed, error) {
	if cacheItem, found := c.cache.Get(bagId); found {
		return cacheItem.(*BagDetailed), nil
	}

	resp, err := c.svc.GetBag(ctx, bagId)
	if err != nil {
		return nil, err
	}

	c.cache.Set(bagId, resp)

	return resp, nil
}

func (c *cacheMiddleware) StartDownload(ctx context.Context, bagId string, downloadAll bool) error {
	return c.svc.StartDownload(ctx, bagId, downloadAll)
}

func (c *cacheMiddleware) RemoveBag(ctx context.Context, bagId string, withFiles bool) error {
	return c.svc.RemoveBag(ctx, bagId, withFiles)
}

func NewCacheMiddleware(svc Client) *cacheMiddleware {
	return &cacheMiddleware{
		cache: cache.NewSimpleCache(time.Hour * 24),
		svc:   svc,
	}
}
