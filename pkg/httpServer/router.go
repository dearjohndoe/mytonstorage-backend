//go:build !debug

package httpServer

import (
	"time"

	"github.com/gofiber/fiber/v2/middleware/limiter"
)

const (
	MaxRequests     = 30
	RateLimitWindow = 60 * time.Second
)

func (h *handler) RegisterRoutes() {
	h.logger.Info("Registering routes")

	m := newMetrics(h.namespace, h.subsystem)

	h.server.Use(m.metricsMiddleware)

	h.server.Use(limiter.New(limiter.Config{
		Max:               MaxRequests,
		Expiration:        RateLimitWindow,
		LimitReached:      h.limitReached,
		LimiterMiddleware: limiter.SlidingWindow{},
	}))

	h.server.Get("/health", h.health)
	h.server.Get("/metrics", h.adminAuthMiddleware, h.metrics)

	apiv1 := h.server.Group("/api/v1", h.loggerMiddleware)
	{
		{
			auth := apiv1.Group("")
			auth.Post("/login", h.login)
			auth.Get("/ton-proof", h.getData)
		}

		{
			files := apiv1.Group("/files", h.userAuthMiddleware)
			files.Post("/", h.uploadFiles)
<<<<<<< Updated upstream
			files.Get("/:bag_id", h.bagInfo)
=======
			files.Post("/paid", h.markBagAsPaid)
			files.Post("/unpaid", h.getUnpaid)
>>>>>>> Stashed changes
			files.Delete("/:bag_id", h.deleteBag)
			files.Get("/:bag_id", h.bagInfo)
		}

		{
			providers := apiv1.Group("/providers", h.userAuthMiddleware)
			providers.Post("/offers", h.fetchProvidersOffers)
			providers.Post("/init-contract", h.initStorageContract)
		}
	}
}
