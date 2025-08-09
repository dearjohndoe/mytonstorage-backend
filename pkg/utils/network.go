package utils

import (
	"encoding/hex"
	"fmt"
)

func ToHashBytes(hash string) ([]byte, error) {
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash hex string")
	}

	if len(hashBytes) != 32 {
		return nil, fmt.Errorf("invalid hash size, length should be 64 symbols")
	}
	return hashBytes, nil
}
