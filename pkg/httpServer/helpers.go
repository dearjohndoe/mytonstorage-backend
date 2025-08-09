package httpServer

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"mytonstorage-backend/pkg/models"
)

func (h *handler) limitReached(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", "limitReached"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	log.Warn("rate limit reached for request")
	return fiber.NewError(fiber.StatusTooManyRequests, "too many requests, please try again later")
}

func validateBagID(bagid string) bool {
	if len(bagid) != 64 {
		return false
	}

	for i := range 64 {
		c := bagid[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}

func okHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
	})
}

func errorHandler(c *fiber.Ctx, err error) error {
	if e, ok := err.(*fiber.Error); ok {
		return c.Status(e.Code).JSON(fiber.Map{
			"error": e.Message,
		})
	}

	if appErr, ok := err.(*models.AppError); ok {
		msg := appErr.Message
		if appErr.Code > 500 {
			msg = "internal server error"
		}

		return c.Status(appErr.Code).JSON(fiber.Map{
			"error": msg,
		})
	}

	errorResponse := errorResponse{
		Error: err.Error(),
	}

	return c.Status(fiber.StatusInternalServerError).JSON(errorResponse)
}
