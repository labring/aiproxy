package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	// import gif decoder
	_ "image/gif"
	// import jpeg decoder
	_ "image/jpeg"
	// import png decoder
	_ "image/png"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/labring/aiproxy/core/common"
	// import webp decoder
	_ "golang.org/x/image/webp"
)

// Regex to match data URL pattern
var dataURLPattern = regexp.MustCompile(`^data:image/([^;]+);base64,(.*)`)

func IsImageURL(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

// TrimImageContentType delete after `;`
func TrimImageContentType(contentType string) string {
	before, _, _ := strings.Cut(contentType, ";")
	return before
}

func GetImageSizeFromURL(url string) (width, height int, err error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return width, height, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("download image error: status code: %d", resp.StatusCode)
	}

	if resp.ContentLength > MaxImageSize {
		return 0, 0, fmt.Errorf(
			"download image error: image too large: %d, max: %d",
			resp.ContentLength,
			MaxImageSize,
		)
	}

	var reader io.Reader
	if resp.ContentLength <= 0 {
		reader = common.LimitReader(resp.Body, MaxImageSize)
	} else {
		reader = resp.Body
	}

	img, _, err := image.DecodeConfig(reader)
	if err != nil {
		return width, height, err
	}

	return img.Width, img.Height, nil
}

const (
	MaxImageSize = 1024 * 1024 * 10 // 10MB
)

func GetImageFromURL(ctx context.Context, url string) (string, string, error) {
	// Check if the URL is a data URL
	if !strings.HasPrefix(url, "http://") &&
		!strings.HasPrefix(url, "https://") {
		matches := dataURLPattern.FindStringSubmatch(url)
		if len(matches) == 3 {
			return "image/" + matches[1], matches[2], nil
		}

		return "", "", errors.New("download image error: not an image url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download image error: status code: %d", resp.StatusCode)
	}

	buf, err := common.GetResponseBodyLimit(resp, MaxImageSize)
	if err != nil {
		return "", "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if !IsImageURL(contentType) {
		contentType = http.DetectContentType(buf)
		if !IsImageURL(contentType) {
			return "", "", errors.New("download iamge error: not an image")
		}
	}

	return TrimImageContentType(contentType), base64.StdEncoding.EncodeToString(buf), nil
}

var reg = regexp.MustCompile(`^data:image/([^;]+);base64,`)

func GetImageSizeFromBase64(encoded string) (width, height int, err error) {
	decoded, err := base64.StdEncoding.DecodeString(reg.ReplaceAllString(encoded, ""))
	if err != nil {
		return 0, 0, err
	}

	img, _, err := image.DecodeConfig(bytes.NewReader(decoded))
	if err != nil {
		return 0, 0, err
	}

	return img.Width, img.Height, nil
}

func GetImageSize(image string) (width, height int, err error) {
	if strings.HasPrefix(image, "data:image/") {
		return GetImageSizeFromBase64(image)
	}
	return GetImageSizeFromURL(image)
}
