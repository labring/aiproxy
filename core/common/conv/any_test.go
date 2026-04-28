package conv_test

import (
	"testing"

	"github.com/labring/aiproxy/core/common/conv"
	"github.com/smartystreets/goconvey/convey"
)

func TestBytesToString(t *testing.T) {
	convey.Convey("BytesToString", t, func() {
		convey.Convey("should convert bytes to string", func() {
			b := []byte("hello")
			s := conv.BytesToString(b)
			convey.So(s, convey.ShouldEqual, "hello")
		})
	})
}

func TestStringToBytes(t *testing.T) {
	convey.Convey("StringToBytes", t, func() {
		convey.Convey("should convert string to bytes", func() {
			s := "hello"
			b := conv.StringToBytes(s)
			convey.So(b, convey.ShouldResemble, []byte("hello"))
		})
	})
}
