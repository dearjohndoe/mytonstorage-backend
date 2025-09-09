package system

import (
	"context"
	"time"

	"mytonstorage-backend/pkg/cache"
)

type cacheMiddleware struct {
	svc   Repository
	cache *cache.SimpleCache
}

func (f *cacheMiddleware) SetParam(ctx context.Context, key string, value string) (err error) {
	err = f.svc.SetParam(ctx, key, value)
	if err != nil {
		return
	}

	f.cache.Set(key, value)

	return
}

func (f *cacheMiddleware) GetParam(ctx context.Context, key string) (value string, err error) {
	if value, ok := f.cache.Get(key); ok && value.(string) != "" {
		return value.(string), nil
	}

	return f.svc.GetParam(ctx, key)
}

func NewCacheMiddleware(
	svc Repository,
) Repository {
	return &cacheMiddleware{
		svc:   svc,
		cache: cache.NewSimpleCache(1 * time.Minute),
	}
}
