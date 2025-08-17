package utils

import (
	"encoding/hex"
	"fmt"
	"time"
)

func TryNTimes(f func() error, n int) (err error) {
	for i := 0; i < n; i++ {
		err = f()
		if err == nil {
			return nil
		}

		time.Sleep(time.Second)
	}
	return err
}

func HexToString(b []byte) string {
	return fmt.Sprintf("%x", b)
}

func StringToHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

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
