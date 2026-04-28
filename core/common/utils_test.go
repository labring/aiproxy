package common_test

import (
	"testing"

	"github.com/labring/aiproxy/core/common"
	"github.com/smartystreets/goconvey/convey"
)

func TestShortUUID(t *testing.T) {
	convey.Convey("ShortUUID", t, func() {
		convey.Convey("should return a 32-character hex string", func() {
			uid := common.ShortUUID()
			convey.So(len(uid), convey.ShouldEqual, 32)
		})

		convey.Convey("should be unique", func() {
			uid1 := common.ShortUUID()
			uid2 := common.ShortUUID()
			convey.So(uid1, convey.ShouldNotEqual, uid2)
		})
	})
}
