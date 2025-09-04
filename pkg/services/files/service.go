package files

import (
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xssnick/tonutils-go/address"

	tonstorage "mytonstorage-backend/pkg/clients/ton-storage"
	"mytonstorage-backend/pkg/models"
	v1 "mytonstorage-backend/pkg/models/api/v1"
	"mytonstorage-backend/pkg/models/db"
)

const (
	descriptionsStoreLimit = 1000
)

type service struct {
	files               filesDb
	tonstorage          storage
	storageDir          string
	unpaidFilesLifetime time.Duration
	logger              *slog.Logger
}

type storage interface {
	Create(ctx context.Context, description, path string) (string, error)
	GetBag(ctx context.Context, bagId string) (*tonstorage.BagDetailed, error)
	RemoveBag(ctx context.Context, bagId string, withFiles bool) error
}

type filesDb interface {
	AddBag(ctx context.Context, bag db.BagInfo, userAddr string) error
	RemoveUserBagRelation(ctx context.Context, bagID, userAddress string) (int64, error)
	RemoveUnusedBags(ctx context.Context) (removed []string, err error)
	GetUnpaidBags(ctx context.Context, userID string) ([]db.UserBagInfo, error)
	GetNotifyInfo(ctx context.Context, limit int, notifyAttempts int) ([]db.BagStorageContract, error)
	IncreaseAttempts(ctx context.Context, bags []db.BagStorageContract) error
	MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (cnt int64, err error)
	GetBagsInfoShort(ctx context.Context, contracts []string) (info []db.BagDescription, err error)
}

type Files interface {
	AddFiles(ctx context.Context, description string, file []*multipart.FileHeader, userAddr string) (bagid string, err error)
	DeleteBag(ctx context.Context, bagID string, userAddr string) error
	MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (err error)
	GetUnpaidBags(ctx context.Context, userAddr string) (info v1.UnpaidBagsResponse, err error)
	GetBagsInfoShort(ctx context.Context, contracts []string) (info []v1.BagInfoShort, err error)
}

func (s *service) AddFiles(ctx context.Context, description string, files []*multipart.FileHeader, userAddr string) (bagid string, err error) {
	log := s.logger.With(
		slog.String("method", "AddFiles"),
		slog.String("description", description),
		slog.Int("file_count", len(files)),
	)

	unpaid, err := s.files.GetUnpaidBags(ctx, userAddr)
	if err != nil {
		log.Error("Failed to get unpaid bags", slog.Any("error", err))
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	if len(unpaid) > 0 {
		err = models.NewAppError(models.BadRequestErrorCode, "you have unpaid bags")
		return
	}

	id, uErr := uuid.NewV6()
	if uErr != nil {
		log.Error("Failed to generate UUID", slog.Any("error", uErr))
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	dstPath := filepath.Join(s.storageDir, id.String())
	if oErr := os.MkdirAll(dstPath, 0755); oErr != nil {
		log.Error("Failed to create directory", slog.Any("error", oErr))
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	// Remove the directory if handling an error
	defer func() {
		if err != nil {
			if rmErr := os.RemoveAll(dstPath); rmErr != nil {
				log.Error("Failed to remove directory after error", slog.Any("error", rmErr))
			}
		}
	}()

	// Save files to disk
	rootDir := ""
	for _, f := range files {
		src, fErr := f.Open()
		if fErr != nil {
			log.Error("Failed to open uploaded file", slog.Any("error", fErr))
			err = models.NewAppError(models.InternalServerErrorCode, "")
			return
		}
		defer src.Close()

		fileName := f.Filename
		cd := f.Header.Get("Content-Disposition")
		parts := strings.Split(cd, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "filename=") {
				fileName = strings.Trim(part[len("filename="):], "\"")
				break
			}
		}

		if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
			if rootDir == "" {
				rootDir = filepath.Dir(fileName)
				if i := strings.Index(rootDir, "/"); i != -1 {
					rootDir = rootDir[:i]
				}
			}

			subDir := filepath.Join(dstPath, filepath.Dir(fileName))
			if err := os.MkdirAll(subDir, 0755); err != nil {
				log.Error("Failed to create subdirectory", slog.Any("error", err))
				return "", models.NewAppError(models.InternalServerErrorCode, "")
			}
		}

		dst, cErr := os.Create(filepath.Join(dstPath, fileName))
		if cErr != nil {
			log.Error("Failed to create file on disk", slog.Any("error", cErr))
			err = models.NewAppError(models.InternalServerErrorCode, "")
			return
		}
		defer dst.Close()

		_, cErr = io.Copy(dst, src)
		if cErr != nil {
			log.Error("Failed to copy file to disk", slog.Any("error", cErr))
			err = models.NewAppError(models.InternalServerErrorCode, "")
			return
		}
	}

	// Save file(s) to TON Storage
	path := filepath.Join(dstPath, rootDir)
	if len(files) == 1 && rootDir == "" {
		path = filepath.Join(path, files[0].Filename)
	}
	if path == "" {
		path = dstPath
	}

	bagid, err = s.tonstorage.Create(ctx, description, path)
	if err != nil {
		log.Error("Failed to create file in storage", slog.Any("error", err))
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	bagInfo, err := s.tonstorage.GetBag(ctx, bagid)
	if err != nil {
		log.Error("Failed to get bag info", "error", err.Error())
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	// Save bag info to database
	err = s.files.AddBag(ctx, db.BagInfo{
		BagID:       bagid,
		Description: description,
		Size:        bagInfo.BagSize,
		FilesSize:   bagInfo.Size,
	}, userAddr)
	if err != nil {
		log.Error("Failed to save bag info to database", "error", err.Error())
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	log.Info("File added successfully", slog.String("bag_id", bagid))

	return
}

func (s *service) DeleteBag(ctx context.Context, bagID string, userAddr string) error {
	log := s.logger.With(
		slog.String("method", "DeleteBag"),
		slog.String("bag_id", bagID),
	)

	_, err := s.files.RemoveUserBagRelation(ctx, bagID, userAddr)
	if err != nil {
		log.Error("Failed to remove bag relation", "error", err)
		return models.NewAppError(models.InternalServerErrorCode, "")
	}

	// NOTE: File will be removed automatically by RemoveUnpaidFiles worker
	log.Info("Bag marked to be deleted successfully")

	return nil
}

func (s *service) MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (err error) {
	log := s.logger.With(
		slog.String("method", "MarkBagAsPaid"),
		slog.String("bag_id", bagID),
	)

	addr, err := address.ParseAddr(storageContract)
	if err != nil {
		log.Error("Failed to parse storage contract address", "error", err)
		return models.NewAppError(models.BadRequestErrorCode, "invalid contract address")
	}

	_, err = s.files.MarkBagAsPaid(ctx, bagID, userAddress, addr.String())
	if err != nil {
		log.Error("Failed to mark bag as paid", "error", err)
		return models.NewAppError(models.InternalServerErrorCode, "")
	}

	log.Info("Bag deleted by user successfully", slog.String("bag_id", bagID))
	return nil
}

func (s *service) GetUnpaidBags(ctx context.Context, userAddr string) (info v1.UnpaidBagsResponse, err error) {
	log := s.logger.With(
		slog.String("method", "GetUnpaidBags"),
		slog.String("user_address", userAddr),
	)

	unpaidBags, err := s.files.GetUnpaidBags(ctx, userAddr)
	if err != nil {
		log.Error("Failed to get unpaid bags", "error", err)
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return
	}

	info.Bags = make([]v1.UserBagInfo, 0, len(unpaidBags))
	for _, bag := range unpaidBags {
		bagDetails, sErr := s.tonstorage.GetBag(ctx, bag.BagID)
		if sErr != nil {
			log.Error("Failed to get bag details", slog.Any("error", sErr))
			continue
		}

		info.Bags = append(info.Bags, v1.UserBagInfo{
			BagID:       bag.BagID,
			UserAddress: bag.UserAddress,
			CreatedAt:   bag.CreatedAt,
			Description: bagDetails.Description,
			FilesCount:  bagDetails.FilesCount,
			BagSize:     bagDetails.BagSize,
		})
	}

	info.FreeStorage = uint64(s.unpaidFilesLifetime.Seconds())

	return
}

func (s *service) GetBagsInfoShort(ctx context.Context, contracts []string) (info []v1.BagInfoShort, err error) {
	log := s.logger.With(
		slog.String("method", "GetBagsInfoShort"),
		slog.Int("bag_ids_count", len(contracts)),
	)

	if len(contracts) > descriptionsStoreLimit {
		contracts = contracts[:descriptionsStoreLimit]
	}

	desc, err := s.files.GetBagsInfoShort(ctx, contracts)
	if err != nil {
		log.Error("Failed to get bag descriptions", "error", err)
		return nil, models.NewAppError(models.InternalServerErrorCode, "")
	}

	info = make([]v1.BagInfoShort, 0, len(desc))
	for _, d := range desc {
		info = append(info, v1.BagInfoShort{
			ContractAddress: d.ContractAddress,
			BagID:           d.BagID,
			Description:     d.Description,
			Size:            d.Size,
		})
	}

	return info, nil
}

func NewService(
	files filesDb,
	storage storage,
	storageDir string,
	unpaidFilesLifetime time.Duration,
	logger *slog.Logger,
) Files {
	return &service{
		files:               files,
		tonstorage:          storage,
		storageDir:          storageDir,
		unpaidFilesLifetime: unpaidFilesLifetime,
		logger:              logger,
	}
}
