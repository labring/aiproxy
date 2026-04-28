package utils_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/smartystreets/goconvey/convey"
)

func TestParsePageParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	convey.Convey("ParsePageParams", t, func() {
		convey.Convey("should parse valid page and per_page", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = &http.Request{
				URL: &url.URL{
					RawQuery: "page=2&per_page=20",
				},
			}

			page, perPage := utils.ParsePageParams(c)
			convey.So(page, convey.ShouldEqual, 2)
			convey.So(perPage, convey.ShouldEqual, 20)
		})

		convey.Convey("should parse 'p' alias for page", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = &http.Request{
				URL: &url.URL{
					RawQuery: "p=3&per_page=15",
				},
			}

			page, perPage := utils.ParsePageParams(c)
			convey.So(page, convey.ShouldEqual, 3)
			convey.So(perPage, convey.ShouldEqual, 15)
		})

		convey.Convey("should return 0 for missing params", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = &http.Request{
				URL: &url.URL{},
			}

			page, perPage := utils.ParsePageParams(c)
			convey.So(page, convey.ShouldEqual, 0)
			convey.So(perPage, convey.ShouldEqual, 0)
		})
	})
}

func TestParseTimeRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	convey.Convey("ParseTimeRange", t, func() {
		now := time.Now()
		startTimeStr := strconv.FormatInt(now.Add(-time.Hour).Unix(), 10)
		endTimeStr := strconv.FormatInt(now.Unix(), 10)

		convey.Convey("should parse valid timestamps", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = &http.Request{
				URL: &url.URL{
					RawQuery: "start_timestamp=" + startTimeStr + "&end_timestamp=" + endTimeStr,
				},
			}

			start, end := utils.ParseTimeRange(c, 0)
			// Timestamps are in seconds, so we check if they are roughly equal (ignoring sub-second differences lost in conversion)
			convey.So(start.Unix(), convey.ShouldEqual, now.Add(-time.Hour).Unix())
			convey.So(end.Unix(), convey.ShouldEqual, now.Unix())
		})

		convey.Convey("should use default max span if start time is too old", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Start time 10 days ago
			oldStartStr := strconv.FormatInt(now.Add(-24*10*time.Hour).Unix(), 10)

			c.Request = &http.Request{
				URL: &url.URL{
					RawQuery: "start_timestamp=" + oldStartStr,
				},
			}

			start, end := utils.ParseTimeRange(c, time.Hour*24*7) // Max 7 days

			// End should be effectively Now() (since not provided)
			// Start should be End - 7 days
			expectedStart := end.Add(-time.Hour * 24 * 7)

			convey.So(end.Unix(), convey.ShouldAlmostEqual, now.Unix(), 5) // Allow 5s diff
			convey.So(start.Unix(), convey.ShouldAlmostEqual, expectedStart.Unix(), 5)
		})

		convey.Convey("should handle millisecond timestamps", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			milliStart := now.Add(-time.Hour).UnixMilli()
			milliEnd := now.UnixMilli()

			c.Request = &http.Request{
				URL: &url.URL{
					RawQuery: "start_timestamp=" + strconv.FormatInt(
						milliStart,
						10,
					) + "&end_timestamp=" + strconv.FormatInt(
						milliEnd,
						10,
					),
				},
			}

			start, end := utils.ParseTimeRange(c, 0)
			convey.So(start.Unix(), convey.ShouldEqual, now.Add(-time.Hour).Unix())
			convey.So(end.Unix(), convey.ShouldEqual, now.Unix())
		})
	})
}
