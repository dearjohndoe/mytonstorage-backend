package files

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mytonstorage-backend/pkg/constants"
)

const (
	maxFilesCount = "max_files_count"
)

func sanitizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	cleaned := filepath.Clean(p)

	if cleaned == "." {
		return ".", nil
	}

	if len(cleaned) > constants.MaxPathLength {
		return "", errors.New("path too long")
	}

	segments := strings.Split(cleaned, string(filepath.Separator))
	for _, seg := range segments {
		if seg == ".." {
			return "", errors.New("invalid segment in path")
		}
	}

	if strings.ContainsRune(cleaned, '\x00') {
		return "", errors.New("invalid path")
	}

	return cleaned, nil
}

func (s *service) validateAvailableSpace(ctx context.Context, filesSize uint64) error {
	list, err := s.tonstorage.List(ctx)
	if err != nil {
		return err
	}

	usedSpace := uint64(0)
	for _, bag := range list.Bags {
		usedSpace += bag.Size
	}

	if usedSpace+filesSize > s.totalDiskSpaceAvailable {
		return errors.New("not enough disk space available")
	}

	return nil
}

func (s *service) getLimits(ctx context.Context) (maxCount int, err error) {
	countStr, err := s.system.GetParam(ctx, maxFilesCount)
	if err != nil {
		return 0, err
	}
	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func saveFileToDisk(dstPath, fileName string, file []byte) (err error) {
	if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		subDir := filepath.Join(dstPath, filepath.Dir(fileName))
		if err := os.MkdirAll(subDir, 0755); err != nil {
			return err
		}
	}

	dst, cErr := os.Create(filepath.Join(dstPath, fileName))
	if cErr != nil {
		return err
	}
	defer dst.Close()

	_, err = dst.Write(file)
	if err != nil {
		return err
	}

	return nil
}
