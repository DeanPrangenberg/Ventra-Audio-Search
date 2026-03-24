package globalUtils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func StringSha256Hex(str string) string {
	return ByteSha256Hex([]byte(str))
}

func ByteSha256Hex(data []byte) string {
	sum := sha256.Sum256(data)        // [32]byte
	return hex.EncodeToString(sum[:]) // 64 hex chars
}

func FileSha256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
