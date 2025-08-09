package httpServer

import (
	"context"
	"log/slog"
	"mime/multipart"

	"github.com/gofiber/fiber/v2"

	v1 "mytonstorage-backend/pkg/models/api/v1"
)

type files interface {
	AddFiles(ctx context.Context, description string, file []*multipart.FileHeader) (bagid string, err error)
	BagInfo(ctx context.Context, bagID string) (info *v1.BagInfo, err error)
	DeleteBag(ctx context.Context, bagID string) error
}

type providers interface {
	FetchProvidersRates(ctx context.Context, req v1.OffersRequest) (resp v1.ProviderRatesResponse, err error)
	InitStorageContract(ctx context.Context, info v1.InitStorageContractRequest, providers []v1.ProviderShort) (resp v1.Transaction, err error)
}

type auth interface {
	GetData() string
	Login(ctx context.Context, info v1.LoginInfo) (sessionID string, err error)
	Authenticate(ctx context.Context, signature, sessionData string) (addr string, err error)
}

type errorResponse struct {
	Error string `json:"error"`
}

type handler struct {
	server          *fiber.App
	logger          *slog.Logger
	files           files
	providers       providers
	auth            auth
	namespace       string
	subsystem       string
	adminAuthTokens map[string]struct{}
}

func New(
	server *fiber.App,
	files files,
	providers providers,
	auth auth,
	adminAuthTokens []string,
	namespace string,
	subsystem string,
	logger *slog.Logger,
) *handler {
	adminTokensMap := make(map[string]struct{})
	for _, token := range adminAuthTokens {
		adminTokensMap[token] = struct{}{}
	}

	h := &handler{
		server:          server,
		files:           files,
		providers:       providers,
		auth:            auth,
		namespace:       namespace,
		subsystem:       subsystem,
		adminAuthTokens: adminTokensMap,
		logger:          logger,
	}

	return h
}
