package common_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/smartystreets/goconvey/convey"
)

func TestGetLogFields(t *testing.T) {
	convey.Convey("GetLogFields", t, func() {
		fields := common.GetLogFields()
		convey.So(fields, convey.ShouldNotBeNil)
		convey.So(len(fields), convey.ShouldEqual, 0)

		fields["test"] = "value"
		common.PutLogFields(fields)

		// Should get a cleared map (or at least we should be able to reuse it)
		fields2 := common.GetLogFields()
		convey.So(fields2, convey.ShouldNotBeNil)
		convey.So(len(fields2), convey.ShouldEqual, 0)
	})
}

func TestLogger(t *testing.T) {
	convey.Convey("Logger Context", t, func() {
		convey.Convey("GetLoggerFromReq should create new logger if missing", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			logger := common.GetLoggerFromReq(req)
			convey.So(logger, convey.ShouldNotBeNil)
			convey.So(logger.Data, convey.ShouldNotBeNil)
		})

		convey.Convey("SetLogger should store logger in context", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			entry := common.NewLogger()
			common.SetLogger(req, entry)

			logger := common.GetLoggerFromReq(req)
			convey.So(logger, convey.ShouldEqual, entry)
		})

		convey.Convey("GetLogger should work with Gin context", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			logger := common.GetLogger(c)
			convey.So(logger, convey.ShouldNotBeNil)
		})
	})
}

func TestTruncateDuration(t *testing.T) {
	convey.Convey("TruncateDuration", t, func() {
		convey.Convey("should truncate > 1h to Minute", func() {
			d := time.Hour + 30*time.Minute + 30*time.Second
			res := common.TruncateDuration(d)
			convey.So(res, convey.ShouldEqual, time.Hour+30*time.Minute)
		})

		convey.Convey("should truncate > 1m to Second", func() {
			d := time.Minute + 30*time.Second + 500*time.Millisecond
			res := common.TruncateDuration(d)
			convey.So(res, convey.ShouldEqual, time.Minute+30*time.Second)
		})

		convey.Convey("should truncate > 1s to Millisecond", func() {
			d := time.Second + 500*time.Millisecond + 500*time.Microsecond
			res := common.TruncateDuration(d)
			convey.So(res, convey.ShouldEqual, time.Second+500*time.Millisecond)
		})

		convey.Convey("should truncate > 1ms to Microsecond", func() {
			d := time.Millisecond + 500*time.Microsecond + 500*time.Nanosecond
			res := common.TruncateDuration(d)
			convey.So(res, convey.ShouldEqual, time.Millisecond+500*time.Microsecond)
		})

		convey.Convey("should keep small durations", func() {
			d := 500 * time.Nanosecond
			res := common.TruncateDuration(d)
			convey.So(res, convey.ShouldEqual, d)
		})

		convey.Convey("should handle exact boundaries", func() {
			// 1h -> falls through to > 1m check (since 1h is not > 1h)
			// returns 1h truncated to Second -> 1h
			convey.So(common.TruncateDuration(time.Hour), convey.ShouldEqual, time.Hour)

			convey.So(common.TruncateDuration(time.Minute), convey.ShouldEqual, time.Minute)
			convey.So(common.TruncateDuration(time.Second), convey.ShouldEqual, time.Second)
			convey.So(
				common.TruncateDuration(time.Millisecond),
				convey.ShouldEqual,
				time.Millisecond,
			)
		})
	})
}
