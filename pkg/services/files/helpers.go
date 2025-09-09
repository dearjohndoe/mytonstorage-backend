package files

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
)

const (
	maxFilesSize  = "max_files_size"
	maxFilesCount = "max_files_count"
)

func (s *service) validate(description string, files []*multipart.FileHeader, maxSize uint64, maxCount int) error {
	if len(files) == 0 {
		return errors.New("no files provided")
	}

	if len(files) > maxCount {
		s := fmt.Sprintf("too many files, max %d files allowed", maxCount)
		return errors.New(s)
	}

	if len(description) > 100 {
		return errors.New("description too long, max 100 characters")
	}

	totalSize := uint64(0)
	for _, file := range files {
		totalSize += uint64(file.Size)
	}

	if totalSize > maxSize {
		return errors.New("total file size too large, max 100MB")
	}

	return nil
}

func (s *service) validateAvailableSpace(ctx context.Context, availableDiskSpace uint64) error {
	list, err := s.tonstorage.List(ctx)
	if err != nil {
		return err
	}

	usedSpace := uint64(0)
	for _, bag := range list.Bags {
		usedSpace += bag.Size
	}

	if usedSpace > availableDiskSpace {
		return errors.New("not enough disk space available")
	}

	return nil
}

func (s *service) getLimits(ctx context.Context) (maxSize uint64, maxCount int, err error) {
	sizeStr, err := s.system.GetParam(ctx, maxFilesSize)
	if err != nil {
		return 0, 0, err
	}
	size, err := strconv.ParseUint(sizeStr, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	countStr, err := s.system.GetParam(ctx, maxFilesCount)
	if err != nil {
		return 0, 0, err
	}
	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	return size, int(count), nil
}
