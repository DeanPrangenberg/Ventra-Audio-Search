package globalUtils

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFileAtomicMP3(data []byte, filename string) (fullPath string, err error) {
	// harden filename (no path traversal)
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" || filename == "" {
		return "", fmt.Errorf("invalid filename")
	}

	fullPath = filepath.Join("/app/downloaded_audios/downloaded", filename+".mp3")
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()

	// cleanup only on error
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	// optional but common:
	if err := tmp.Chmod(0o644); err != nil {
		return "", err
	}

	if _, err := tmp.Write(data); err != nil {
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	// correct destination:
	if err := os.Rename(tmpName, fullPath); err != nil {
		return "", err
	}

	return fullPath, nil
}

func MarkFileAtomicMP3(unmarkedFilePath string) (newFilePath string, hash string, err error) {

	if _, err := os.Stat(unmarkedFilePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("file does not exist: %s", unmarkedFilePath)
	}

	hash, err = FileSha256Hex(unmarkedFilePath)
	if err != nil {
		return "", "", err
	}

	newFile := "/app/downloaded_audios/marked/" + hash + ".mp3"

	dir := filepath.Dir(newFile)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}

	if err := os.Rename(unmarkedFilePath, newFile); err != nil {
		return "", "", err
	}

	return newFile, hash, nil
}
