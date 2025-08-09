package httpServer

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	v1 "mytonstorage-backend/pkg/models/api/v1"
)

func (h *handler) login(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", "login"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	var info v1.LoginInfo
	if err := c.BodyParser(&info); err != nil {
		log.Error("failed to parse login info", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	sessionID, err := h.auth.Login(c.Context(), info)
	if err != nil {
		log.Error("failed to login", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to login")
	}

	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteStrictMode,
	})

	return okHandler(c)
}

func (h *handler) getData(c *fiber.Ctx) error {
	data := h.auth.GetData()

	return c.JSON(fiber.Map{"data": data})
}

func (h *handler) uploadFiles(c *fiber.Ctx) (err error) {
	log := h.logger.With(
		slog.String("method", "uploadFile"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	mp, err := c.MultipartForm()
	if err != nil {
		log.Error("failed to get multipart form", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid multipart form")
	}

	files, ok := mp.File["file"]
	if !ok || len(files) == 0 {
		log.Error("failed to get files from form", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "no files provided")
	}

	var description string
	if data, ok := mp.Value["description"]; ok {
		description = data[0]
	}

	bagid, err := h.files.AddFiles(c.Context(), description, files)
	if err != nil {
		log.Error("failed to add files", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to upload files")
	}

	return c.JSON(fiber.Map{
		"bag_id": bagid,
	})
}

func (h *handler) bagInfo(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", "bagInfo"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	bagID := strings.ToLower(c.Params("bag_id"))
	if !validateBagID(bagID) {
		log.Error("bag_id is required")
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	info, err := h.files.BagInfo(c.Context(), bagID)
	if err != nil {
		log.Error("failed to get bag info", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "")
	}

	return c.JSON(info)
}

func (h *handler) deleteBag(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", "deleteBag"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	bagID := strings.ToLower(c.Params("bag_id"))
	if !validateBagID(bagID) {
		log.Error("bag_id is required")
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	err := h.files.DeleteBag(c.Context(), bagID)
	if err != nil {
		log.Error("failed to delete bag", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete bag")
	}

	return okHandler(c)
}

func (h *handler) fetchProvidersOffers(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", "fetchProvidersOffers"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	var req v1.OffersRequest
	if err := c.BodyParser(&req); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	resp, err := h.providers.FetchProvidersRates(c.Context(), req)
	if err != nil {
		log.Error("failed to fetch providers rates", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to fetch providers rates")
	}

	return c.JSON(resp)
}

func (h *handler) initStorageContract(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", "initStorageContract"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	var info v1.InitStorageContractRequest
	if err := c.BodyParser(&info); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	providersKeys := make([]string, 0, len(info.Providers))
	for _, provider := range info.Providers {
		providersKeys = append(providersKeys, provider.PublicKey)
	}

	rates, err := h.providers.FetchProvidersRates(c.Context(), v1.OffersRequest{
		BagID:     info.BagID,
		Providers: providersKeys,
	})
	if err != nil {
		log.Error("failed to fetch providers rates", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to fetch providers rates")
	}
	if len(rates.Offers) != len(info.Providers) {
		log.Error("not all providers returned offers", slog.Int("expected", len(info.Providers)), slog.Int("received", len(rates.Offers)))
		return fiber.NewError(fiber.StatusBadRequest, "some providers unavailable")
	}

	providersOffers := make([]v1.ProviderShort, 0, len(rates.Offers))
	for _, offer := range rates.Offers {
		index := slices.IndexFunc(info.Providers, func(p v1.ProviderAddress) bool {
			return strings.ToUpper(p.PublicKey) == offer.Provider.Key
		})

		if index == -1 {
			log.Error("provider not found in request", slog.String("provider_key", offer.Provider.Key))
			return fiber.NewError(fiber.StatusBadRequest, "provider not found in request")
		}

		providersOffers = append(providersOffers, v1.ProviderShort{
			Address:       info.Providers[index].Address,
			MaxSpan:       offer.OfferSpan,
			PricePerMBDay: offer.PricePerMB,
		})
	}

	resp, err := h.providers.InitStorageContract(c.Context(), info, providersOffers)
	if err != nil {
		log.Error("failed to init storage contract", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to init storage contract")
	}

	return c.JSON(resp)
}

func (h *handler) health(c *fiber.Ctx) error {
	return okHandler(c)
}

func (h *handler) metrics(c *fiber.Ctx) error {
	m := promhttp.Handler()

	return adaptor.HTTPHandler(m)(c)
}
