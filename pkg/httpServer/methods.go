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
		return errorHandler(c, err)
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
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	address, ok := c.Context().UserValue("address").(string)
	if !ok || address == "" {
		log.Error("no user address after successful auth")
		return fiber.NewError(fiber.StatusInternalServerError, "")
	}

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

	bagid, err := h.files.AddFiles(c.Context(), description, files, address)
	if err != nil {
		return errorHandler(c, err)
	}

	return c.JSON(fiber.Map{
		"bag_id": bagid,
	})
}

func (h *handler) deleteBag(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	address, ok := c.Context().UserValue("address").(string)
	if !ok || address == "" {
		log.Error("no user address after successful auth")
		return fiber.NewError(fiber.StatusInternalServerError, "")
	}

	bagID := strings.ToLower(c.Params("bag_id"))
	if !validateBagID(bagID) {
		log.Error("bag_id is required")
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	err := h.files.DeleteBag(c.Context(), bagID, address)
	if err != nil {
		return errorHandler(c, err)
	}

	return okHandler(c)
}

func (h *handler) getUnpaid(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	address, ok := c.Context().UserValue("address").(string)
	if !ok || address == "" {
		log.Error("no user address after successful auth")
		return fiber.NewError(fiber.StatusInternalServerError, "")
	}

	bagsInfo, err := h.files.GetUnpaidBags(c.Context(), address)
	if err != nil {
		return errorHandler(c, err)
	}

	return c.JSON(bagsInfo)
}

func (h *handler) markBagAsPaid(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	address, ok := c.Context().UserValue("address").(string)
	if !ok || address == "" {
		log.Error("no user address after successful auth")
		return fiber.NewError(fiber.StatusInternalServerError, "")
	}

	var req v1.PaidBagRequest
	if err := c.BodyParser(&req); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	req.BagID = strings.ToLower(req.BagID)
	err := h.files.MarkBagAsPaid(c.Context(), req.BagID, address, req.StorageContract)
	if err != nil {
		return errorHandler(c, err)
	}

	return okHandler(c)
}

func (h *handler) GetBagsInfoShort(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	var req v1.DetailsRequest
	if err := c.BodyParser(&req); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	bagsInfo, err := h.files.GetBagsInfoShort(c.Context(), req.ContractsAddresses)
	if err != nil {
		return errorHandler(c, err)
	}

	return c.JSON(bagsInfo)
}

func (h *handler) fetchProvidersOffers(c *fiber.Ctx) error {
	log := h.logger.With(
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
		return errorHandler(c, err)
	}

	return c.JSON(resp)
}

func (h *handler) topupBalance(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	address, ok := c.Context().UserValue("address").(string)
	if !ok || address == "" {
		log.Error("no user address after successful auth")
		return fiber.NewError(fiber.StatusInternalServerError, "")
	}

	var req v1.TopupRequest
	if err := c.BodyParser(&req); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	resp, err := h.contracts.TopupBalance(c.Context(), address, req)
	if err != nil {
		return errorHandler(c, err)
	}

	return c.JSON(resp)
}

func (h *handler) withdrawBalance(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	address, ok := c.Context().UserValue("address").(string)
	if !ok || address == "" {
		log.Error("no user address after successful auth")
		return fiber.NewError(fiber.StatusInternalServerError, "")
	}

	var req v1.WithdrawRequest
	if err := c.BodyParser(&req); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	resp, err := h.contracts.WithdrawBalance(c.Context(), address, req)
	if err != nil {
		return errorHandler(c, err)
	}

	return c.JSON(resp)
}

func (h *handler) updateProviders(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	var req v1.UpdateProvidersRequest
	if err := c.BodyParser(&req); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	rates := h.providers.FetchProvidersRatesBySize(c.Context(), req.BagSize, req.Providers)
	if len(rates.Offers) != len(req.Providers) {
		log.Error("not all providers returned offers", slog.Int("expected", len(req.Providers)), slog.Int("received", len(rates.Offers)))
		return fiber.NewError(fiber.StatusBadRequest, "some providers unavailable")
	}

	providersOffers := make([]v1.ProviderShort, 0, len(rates.Offers))
	for _, offer := range rates.Offers {
		index := slices.IndexFunc(req.Providers, func(key string) bool {
			return strings.EqualFold(key, offer.Provider.Key)
		})

		if index == -1 {
			log.Error("some providers unavailable", slog.String("provider_key", offer.Provider.Key))
			return fiber.NewError(fiber.StatusBadRequest, "some providers unavailable, please, try again")
		}

		providersOffers = append(providersOffers, v1.ProviderShort{
			Pubkey:        offer.Provider.Key,
			MaxSpan:       offer.OfferSpan,
			PricePerMBDay: offer.PricePerMB,
		})
	}

	resp, err := h.providers.EditStorageContract(c.Context(), req.ContractAddress, req.Amount, providersOffers)
	if err != nil {
		return errorHandler(c, err)
	}

	return c.JSON(resp)
}

func (h *handler) initStorageContract(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
	)

	var info v1.InitStorageContractRequest
	if err := c.BodyParser(&info); err != nil {
		log.Error("failed to parse request", slog.Any("error", err))
		return fiber.NewError(fiber.StatusBadRequest, "invalid request")
	}

	rates, err := h.providers.FetchProvidersRates(c.Context(), v1.OffersRequest{
		BagID:     info.BagID,
		Providers: info.ProvidersKeys,
	})
	if err != nil {
		log.Error("failed to fetch providers rates", slog.Any("error", err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to fetch providers rates")
	}
	if len(rates.Offers) != len(info.ProvidersKeys) {
		log.Error("not all providers returned offers", slog.Int("expected", len(info.ProvidersKeys)), slog.Int("received", len(rates.Offers)))
		return fiber.NewError(fiber.StatusBadRequest, "some providers unavailable")
	}

	providersOffers := make([]v1.ProviderShort, 0, len(rates.Offers))
	for _, offer := range rates.Offers {
		index := slices.IndexFunc(info.ProvidersKeys, func(key string) bool {
			return strings.EqualFold(key, offer.Provider.Key)
		})

		if index == -1 {
			log.Error("some providers unavailable", slog.String("provider_key", offer.Provider.Key))
			return fiber.NewError(fiber.StatusBadRequest, "some providers unavailable, please, try again")
		}

		providersOffers = append(providersOffers, v1.ProviderShort{
			Pubkey:        offer.Provider.Key,
			MaxSpan:       offer.OfferSpan,
			PricePerMBDay: offer.PricePerMB,
		})
	}

	resp, err := h.providers.InitStorageContract(c.Context(), info, providersOffers)
	if err != nil {
		return errorHandler(c, err)
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
