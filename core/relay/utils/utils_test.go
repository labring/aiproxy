package utils_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/smartystreets/goconvey/convey"
)

func TestIsStreamResponseWithHeader(t *testing.T) {
	convey.Convey("IsStreamResponseWithHeader", t, func() {
		convey.Convey("should return true for text/event-stream", func() {
			header := http.Header{}
			header.Set("Content-Type", "text/event-stream")
			convey.So(utils.IsStreamResponseWithHeader(header), convey.ShouldBeTrue)
		})

		convey.Convey("should return true for application/x-ndjson", func() {
			header := http.Header{}
			header.Set("Content-Type", "application/x-ndjson")
			convey.So(utils.IsStreamResponseWithHeader(header), convey.ShouldBeTrue)
		})

		convey.Convey("should return false for application/json", func() {
			header := http.Header{}
			header.Set("Content-Type", "application/json")
			convey.So(utils.IsStreamResponseWithHeader(header), convey.ShouldBeFalse)
		})

		convey.Convey("should return false for empty content type", func() {
			header := http.Header{}
			convey.So(utils.IsStreamResponseWithHeader(header), convey.ShouldBeFalse)
		})
	})
}

func TestScannerBuffer(t *testing.T) {
	convey.Convey("ScannerBuffer", t, func() {
		convey.Convey("should get buffer of correct size", func() {
			buf := utils.GetScannerBuffer()
			convey.So(len(*buf), convey.ShouldEqual, utils.ScannerBufferSize)
			convey.So(cap(*buf), convey.ShouldEqual, utils.ScannerBufferSize)
			utils.PutScannerBuffer(buf)
		})
	})
}

func TestDoRequest(t *testing.T) {
	convey.Convey("DoRequest", t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		defer ts.Close()

		convey.Convey("should make request successfully", func() {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, nil)
			resp, err := utils.DoRequest(req, time.Second)
			convey.So(err, convey.ShouldBeNil)

			defer resp.Body.Close()

			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusOK)
			body, _ := io.ReadAll(resp.Body)
			convey.So(string(body), convey.ShouldEqual, "ok")
		})
	})
}

func TestUnmarshalGeneralOpenAIRequest(t *testing.T) {
	convey.Convey("UnmarshalGeneralOpenAIRequest", t, func() {
		convey.Convey("should unmarshal valid request", func() {
			reqBody := map[string]any{
				"model": "gpt-3.5-turbo",
				"messages": []map[string]string{
					{"role": "user", "content": "hello"},
				},
				"stream": true,
			}
			bodyBytes, _ := json.Marshal(reqBody)
			req, _ := http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/",
				bytes.NewBuffer(bodyBytes),
			)
			req.Header.Set("Content-Type", "application/json")

			parsedReq, err := utils.UnmarshalGeneralOpenAIRequest(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(parsedReq.Model, convey.ShouldEqual, "gpt-3.5-turbo")
			convey.So(parsedReq.Stream, convey.ShouldBeTrue)
			convey.So(len(parsedReq.Messages), convey.ShouldEqual, 1)
			convey.So(parsedReq.Messages[0].Role, convey.ShouldEqual, "user")
		})
	})
}
