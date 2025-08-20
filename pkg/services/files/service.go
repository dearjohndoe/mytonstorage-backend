package files

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/xssnick/tonutils-go/address"

	tonstorage "mytonstorage-backend/pkg/clients/ton-storage"
	"mytonstorage-backend/pkg/models"
	v1 "mytonstorage-backend/pkg/models/api/v1"
	"mytonstorage-backend/pkg/models/db"
)

type service struct {
	files      filesDb
	tonstorage storage
	storageDir string
	logger     *slog.Logger
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
	MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (int64, error)

	GetNotifyInfo(ctx context.Context, limit int, notifyAttempts int) ([]db.BagStorageContract, error)
	IncreaseAttempts(ctx context.Context, bags []db.BagStorageContract) error
}

type Files interface {
	AddFiles(ctx context.Context, description string, file []*multipart.FileHeader, userAddr string) (bagid string, err error)
	BagInfo(ctx context.Context, bagID string) (info *v1.BagInfo, err error)
	DeleteBag(ctx context.Context, bagID string, userAddr string) error
	MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (err error)
	GetUnpaidBags(ctx context.Context, userAddr string) (info []v1.UserBagInfo, err error)
}

func (s *service) AddFiles(ctx context.Context, description string, files []*multipart.FileHeader, userAddr string) (bagid string, err error) {
	log := s.logger.With(
		slog.String("method", "AddFiles"),
		slog.String("description", description),
		slog.Int("file_count", len(files)),
	)

	// todo: check if already has unpaid files

	id, uErr := uuid.NewV6()
	if uErr != nil {
		log.Error("Failed to generate UUID", slog.Any("error", uErr))
		return "", fmt.Errorf("failed to generate UUID: %w", uErr)
	}

	dstPath := filepath.Join(s.storageDir, id.String())
	if err := os.MkdirAll(dstPath, 0755); err != nil {
		log.Error("Failed to create directory", slog.Any("error", err))
		return "", fmt.Errorf("failed to create directory %s: %w", dstPath, err)
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
		src, err := f.Open()
		if err != nil {
			log.Error("Failed to open uploaded file", slog.Any("error", err))
			return "", fmt.Errorf("failed to open file %s: %w", f.Filename, err)
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
				return "", fmt.Errorf("failed to create subdirectory %s: %w", subDir, err)
			}
		}

		dst, err := os.Create(filepath.Join(dstPath, fileName))
		if err != nil {
			log.Error("Failed to create file on disk", slog.Any("error", err))
			return "", fmt.Errorf("failed to create file %s: %w", dstPath, err)
		}
		defer dst.Close()

		_, err = io.Copy(dst, src)
		if err != nil {
			log.Error("Failed to copy file to disk", slog.Any("error", err))
			return "", fmt.Errorf("failed to save file %s: %w", dstPath, err)
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
		return "", err
	}

	bagInfo, err := s.tonstorage.GetBag(ctx, bagid)
	if err != nil {
		log.Error("Failed to get bag info", "error", err.Error())
		return "", models.NewAppError(models.InternalServerErrorCode, "")
	}

	// Save bag info to database
	err = s.files.AddBag(ctx, db.BagInfo{
		BagID:       bagid,
		Description: description,
		Size:        bagInfo.Size,
	}, userAddr)
	if err != nil {
		log.Error("Failed to save bag info to database", "error", err.Error())
		return "", models.NewAppError(models.InternalServerErrorCode, "")
	}

	log.Info("File added successfully", slog.String("bag_id", bagid))

	return bagid, nil
}

func (s *service) BagInfo(ctx context.Context, bagID string) (info *v1.BagInfo, err error) {
	log := s.logger.With(
		slog.String("method", "BagInfo"),
		slog.String("bag_id", bagID),
	)

	bagDetails, err := s.tonstorage.GetBag(ctx, bagID)
	if err != nil {
		log.Error("Failed to get bag details", slog.Any("error", err))
		err = fmt.Errorf("failed to get bag details: %w", err)
		return
	}

	info = &v1.BagInfo{
		BagID:       bagDetails.BagID,
		Description: bagDetails.Description,
		Size:        bagDetails.Size,
		Peers:       len(bagDetails.Peers),
		FilesCount:  bagDetails.FilesCount,
		BagSize:     bagDetails.BagSize,
	}

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

	// NOTE: File will be removed automatically by RemoveUnusedFiles worker
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

func (s *service) GetUnpaidBags(ctx context.Context, userAddr string) (info []v1.UserBagInfo, err error) {
	log := s.logger.With(
		slog.String("method", "GetUnpaidBags"),
		slog.String("user_address", userAddr),
	)

	unpaidBags, err := s.files.GetUnpaidBags(ctx, userAddr)
	if err != nil {
		log.Error("Failed to get unpaid bags", "error", err)
		return nil, models.NewAppError(models.InternalServerErrorCode, "")
	}

	for _, bag := range unpaidBags {
		info = append(info, v1.UserBagInfo{
			BagID:           bag.BagID,
			UserAddress:     bag.UserAddress,
			StorageContract: bag.StorageContract,
			CreatedAt:       bag.CreatedAt,
			UpdatedAt:       bag.UpdatedAt,
		})
	}

	return info, nil
}

func NewService(
	files filesDb,
	storage storage,
	storageDir string,
	logger *slog.Logger,
) Files {
	return &service{
		files:      files,
		tonstorage: storage,
		storageDir: storageDir,
		logger:     logger,
	}
}
