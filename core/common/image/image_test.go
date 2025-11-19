package image_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/common/image"
	"github.com/smartystreets/goconvey/convey"
)

func TestIsImageURL(t *testing.T) {
	convey.Convey("IsImageURL", t, func() {
		convey.Convey("should return true for image content type", func() {
			convey.So(image.IsImageURL("image/jpeg"), convey.ShouldBeTrue)
			convey.So(image.IsImageURL("image/png"), convey.ShouldBeTrue)
		})

		convey.Convey("should return false for non-image content type", func() {
			convey.So(image.IsImageURL("text/plain"), convey.ShouldBeFalse)
			convey.So(image.IsImageURL("application/json"), convey.ShouldBeFalse)
		})
	})
}

func TestTrimImageContentType(t *testing.T) {
	convey.Convey("TrimImageContentType", t, func() {
		convey.Convey("should trim content type", func() {
			convey.So(
				image.TrimImageContentType("image/jpeg; charset=utf-8"),
				convey.ShouldEqual,
				"image/jpeg",
			)
			convey.So(image.TrimImageContentType("image/png"), convey.ShouldEqual, "image/png")
		})
	})
}

func TestGetImageSizeFromBase64(t *testing.T) {
	convey.Convey("GetImageSizeFromBase64", t, func() {
		// 1x1 pixel red dot png
		base64Img := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

		convey.Convey("should get size from base64", func() {
			w, h, err := image.GetImageSizeFromBase64(base64Img)
			convey.So(err, convey.ShouldBeNil)
			convey.So(w, convey.ShouldEqual, 1)
			convey.So(h, convey.ShouldEqual, 1)
		})

		convey.Convey("should return error for invalid base64", func() {
			_, _, err := image.GetImageSizeFromBase64("invalid")
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestGetImageFromURL(t *testing.T) {
	convey.Convey("GetImageFromURL", t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/image.png":
				w.Header().Set("Content-Type", "image/png")
				// 1x1 pixel red dot png
				data, _ := base64.StdEncoding.DecodeString(
					"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
				)
				_, _ = w.Write(data)
			case "/text":
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("hello"))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer ts.Close()

		convey.Convey("should get image from URL", func() {
			mime, data, err := image.GetImageFromURL(context.Background(), ts.URL+"/image.png")
			convey.So(err, convey.ShouldBeNil)
			convey.So(mime, convey.ShouldEqual, "image/png")
			convey.So(
				data,
				convey.ShouldEqual,
				"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
			)
		})

		convey.Convey("should return error for non-image URL", func() {
			_, _, err := image.GetImageFromURL(context.Background(), ts.URL+"/text")
			convey.So(err, convey.ShouldNotBeNil)
		})

		convey.Convey("should return error for 404", func() {
			_, _, err := image.GetImageFromURL(context.Background(), ts.URL+"/404")
			convey.So(err, convey.ShouldNotBeNil)
		})

		convey.Convey("should handle data URL", func() {
			dataURL := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
			mime, data, err := image.GetImageFromURL(context.Background(), dataURL)
			convey.So(err, convey.ShouldBeNil)
			convey.So(mime, convey.ShouldEqual, "image/png")
			convey.So(
				data,
				convey.ShouldEqual,
				"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
			)
		})
	})
}

func TestGetImageSizeFromURL(t *testing.T) {
	convey.Convey("GetImageSizeFromURL", t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/image.png" {
				w.Header().Set("Content-Type", "image/png")
				// 1x1 pixel red dot png
				data, _ := base64.StdEncoding.DecodeString(
					"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
				)
				_, _ = w.Write(data)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer ts.Close()

		convey.Convey("should get image size from URL", func() {
			w, h, err := image.GetImageSizeFromURL(ts.URL + "/image.png")
			convey.So(err, convey.ShouldBeNil)
			convey.So(w, convey.ShouldEqual, 1)
			convey.So(h, convey.ShouldEqual, 1)
		})

		convey.Convey("should return error for 404", func() {
			_, _, err := image.GetImageSizeFromURL(ts.URL + "/404")
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestGetImageSize(t *testing.T) {
	convey.Convey("GetImageSize", t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/image.png" {
				w.Header().Set("Content-Type", "image/png")
				// 1x1 pixel red dot png
				data, _ := base64.StdEncoding.DecodeString(
					"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
				)
				_, _ = w.Write(data)
			}
		}))
		defer ts.Close()

		convey.Convey("should get size from data URL", func() {
			dataURL := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
			w, h, err := image.GetImageSize(dataURL)
			convey.So(err, convey.ShouldBeNil)
			convey.So(w, convey.ShouldEqual, 1)
			convey.So(h, convey.ShouldEqual, 1)
		})

		convey.Convey("should get size from HTTP URL", func() {
			w, h, err := image.GetImageSize(ts.URL + "/image.png")
			convey.So(err, convey.ShouldBeNil)
			convey.So(w, convey.ShouldEqual, 1)
			convey.So(h, convey.ShouldEqual, 1)
		})
	})
}

func TestSVG(t *testing.T) {
	convey.Convey("SVG Decode", t, func() {
		svgContent := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
  <circle cx="50" cy="50" r="40" stroke="green" stroke-width="4" fill="yellow" />
</svg>`

		// Helper to simulate reading from response body or file
		reader := strings.NewReader(svgContent)

		convey.Convey("should decode SVG config", func() {
			// Reset reader
			_, err := reader.Seek(0, 0)
			convey.So(err, convey.ShouldBeNil)
			config, err := image.DecodeConfig(reader)
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Width, convey.ShouldEqual, 100)
			convey.So(config.Height, convey.ShouldEqual, 100)
		})
	})
}
