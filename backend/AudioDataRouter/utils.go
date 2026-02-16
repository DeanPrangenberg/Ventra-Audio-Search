package AudioDataRouter

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/globalUtils"
	"log/slog"
	"time"
)

func saveAudiofileElementToDisk(element globalTypes.AudioDataElement, outChan chan<- globalTypes.AudioDataElement) error {
	if element.FileSavedOnDisk {
		return fmt.Errorf("file with hash %s already saved to disk", element.AudiofileHash)
	}

	hasURL := element.FileUrl != ""
	hasB64 := element.Base64Data != ""

	if !hasURL && !hasB64 {
		return errors.New("tried to save a file without FileUrl or Base64Data")
	}

	initSeed := element.FileUrl
	if initSeed == "" {
		initSeed = element.Base64Data
	}
	initName := globalUtils.StringSha256Hex(initSeed)

	switch {
	case hasURL:
		slog.Info("downloading from url", "url", element.FileUrl)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // mp3 kann groß sein
		defer cancel()

		if err := globalUtils.DownloadURLToFile(ctx, element.FileUrl, initName); err != nil {
			return fmt.Errorf("error while downloading '%s': %w", element.FileUrl, err)
		}

		path, hash, err := globalUtils.MarkFileAtomicMP3(initName)
		if err != nil {
			return fmt.Errorf("error while marking file as mp3 '%s': %w", initName, err)
		}

		element.FileSavedOnDisk = true
		element.DownloadPath = path
		element.AudiofileHash = hash
		element.FileUrl = ""
		element.Base64Data = ""

	case hasB64:
		slog.Info("writing from base64")

		decodedBytes, err := base64.StdEncoding.DecodeString(element.Base64Data)
		if err != nil {
			return fmt.Errorf("base64 decode failed: %w", err)
		}

		filePath, err := globalUtils.WriteFileAtomicMP3(decodedBytes, initName)
		if err != nil {
			return fmt.Errorf("error while writing file '%s': %w", initName, err)
		}

		path, hash, err := globalUtils.MarkFileAtomicMP3(filePath)
		if err != nil {
			return fmt.Errorf("error while marking file as mp3 '%s': %w", initName, err)
		}

		element.FileSavedOnDisk = true
		element.DownloadPath = path
		element.AudiofileHash = hash
		element.Base64Data = ""
		element.FileUrl = ""
	}

	outChan <- element
	return nil
}
