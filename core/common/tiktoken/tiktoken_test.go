package tiktoken_test

import (
	"testing"

	"github.com/labring/aiproxy/core/common/tiktoken"
	"github.com/smartystreets/goconvey/convey"
)

func TestGetTokenEncoder(t *testing.T) {
	convey.Convey("GetTokenEncoder", t, func() {
		convey.Convey("should get encoder for gpt-4o", func() {
			enc := tiktoken.GetTokenEncoder("gpt-4o")
			convey.So(enc, convey.ShouldNotBeNil)
		})

		convey.Convey("should get encoder for gpt-3.5-turbo", func() {
			enc := tiktoken.GetTokenEncoder("gpt-3.5-turbo")
			convey.So(enc, convey.ShouldNotBeNil)
		})

		convey.Convey("should return default encoder for unknown model", func() {
			enc := tiktoken.GetTokenEncoder("unknown-model")
			convey.So(enc, convey.ShouldNotBeNil)
			// Should default to gpt-4o encoder (o200k_base)
			ids, _, _ := enc.Encode("hello")
			convey.So(len(ids), convey.ShouldBeGreaterThan, 0)
		})

		convey.Convey("should cache encoders", func() {
			enc1 := tiktoken.GetTokenEncoder("gpt-4")
			enc2 := tiktoken.GetTokenEncoder("gpt-4")
			convey.So(enc1, convey.ShouldEqual, enc2)
		})
	})
}

func TestEncoding(t *testing.T) {
	convey.Convey("Encoding", t, func() {
		convey.Convey("should encode correctly", func() {
			enc := tiktoken.GetTokenEncoder("gpt-3.5-turbo")
			text := "hello world"
			ids, _, err := enc.Encode(text)
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(ids), convey.ShouldBeGreaterThan, 0)

			decoded, err := enc.Decode(ids)
			convey.So(err, convey.ShouldBeNil)
			convey.So(decoded, convey.ShouldEqual, text)
		})
	})
}
