package files

import (
	"context"
	"mime/multipart"
	"time"

	"mytonstorage-backend/pkg/cache"
	v1 "mytonstorage-backend/pkg/models/api/v1"
)

type cacheMiddleware struct {
	svc   Files
	cache *cache.SimpleCache
}

func (c *cacheMiddleware) AddFiles(ctx context.Context, description string, file []*multipart.FileHeader) (bagid string, err error) {
	return c.svc.AddFiles(ctx, description, file)
}

func (c *cacheMiddleware) BagInfo(ctx context.Context, bagID string) (*v1.BagInfo, error) {
	key := "bag_info:" + bagID

	if cached, found := c.cache.Get(key); found {
		return cached.(*v1.BagInfo), nil
	}

	info, err := c.svc.BagInfo(ctx, bagID)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, info)

	return info, nil
}

func (c *cacheMiddleware) DeleteBag(ctx context.Context, bagID string) (err error) {
	err = c.svc.DeleteBag(ctx, bagID)
	if err != nil {
		return
	}

	key := "bag_info:" + bagID
	c.cache.Release(key)

	return
}

func NewCacheMiddleware(
	svc Files,
) Files {
	return &cacheMiddleware{
		svc:   svc,
		cache: cache.NewSimpleCache(12 * time.Hour),
	}
}
