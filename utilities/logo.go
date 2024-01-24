package utilities

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

func ValidateImage(data string, xMax, yMax, maxSizeInKB int, supportedTypes []string) error {
	imgByteData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("image data base64 decoding failed: %w", err)
	}

	_, mimeType, err := image.Decode(bytes.NewReader(imgByteData))
	if err != nil {
		return fmt.Errorf("image decoding failed: %w", err)
	}

	/*
		x := img.Bounds().Max.X
		y := img.Bounds().Max.Y
		if x > xMax || y > yMax {
			return fmt.Errorf("image dimension cannot be greater than %dx%d pixels", xMax, yMax)
		}
	*/

	size := len(imgByteData)
	maxSizeInB := maxSizeInKB * 1024
	if size > maxSizeInB {
		return fmt.Errorf("image size cannot be greater than %d KB", maxSizeInKB)
	}

	mimeSupported := false
	for _, supportedType := range supportedTypes {
		if mimeType == supportedType {
			mimeSupported = true
			break
		}
	}

	if !mimeSupported {
		return fmt.Errorf("unsupported file type %s, supported are: %s", mimeType, strings.Join(supportedTypes, ","))
	}

	return nil
}
