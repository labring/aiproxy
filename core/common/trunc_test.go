package common_test

import (
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/common"
	"github.com/smartystreets/goconvey/convey"
)

func TestTruncateByRune(t *testing.T) {
	convey.Convey("TruncateByRune", t, func() {
		convey.Convey("should truncate normal string", func() {
			s := "hello world"
			convey.So(common.TruncateByRune(s, 5), convey.ShouldEqual, "hello")
		})

		convey.Convey("should handle chinese characters", func() {
			s := "你好世界"
			// Each chinese char is 3 bytes
			// 5 bytes is not enough for 2 chars (6 bytes), so it should return "你好" which is 6 bytes?
			// Wait, TruncateByRune implementation:
			// for _, r := range s { runeLen := utf8.RuneLen(r) ... total += runeLen }
			// It truncates based on byte length but respecting rune boundaries.

			// "你" (3 bytes)
			// "你好" (6 bytes)
			// TruncateByRune("你好世界", 5)
			// 1. r='你', len=3. total=3 <= 5. ok.
			// 2. r='好', len=3. total=6 > 5. return s[:3] -> "你"
			convey.So(common.TruncateByRune(s, 5), convey.ShouldEqual, "你")
			convey.So(common.TruncateByRune(s, 6), convey.ShouldEqual, "你好")
		})

		convey.Convey("should handle string shorter than length", func() {
			s := "abc"
			convey.So(common.TruncateByRune(s, 10), convey.ShouldEqual, "abc")
		})

		convey.Convey("should handle empty string", func() {
			s := ""
			convey.So(common.TruncateByRune(s, 5), convey.ShouldEqual, "")
		})

		convey.Convey("should handle mixed string", func() {
			s := "a你好"
			// 'a' (1), '你' (3), '好' (3)
			// len 4: 'a' (1) + '你' (3) = 4. Exact.
			convey.So(common.TruncateByRune(s, 4), convey.ShouldEqual, "a你")
			// len 3: 'a' (1) + '你' (3) = 4 > 3. Returns 'a'
			convey.So(common.TruncateByRune(s, 3), convey.ShouldEqual, "a")
		})
	})
}

func TestTruncateBytesByRune(t *testing.T) {
	convey.Convey("TruncateBytesByRune", t, func() {
		convey.Convey("should truncate bytes respecting runes", func() {
			s := "你好世界"
			b := []byte(s)
			// Same logic as string
			convey.So(string(common.TruncateBytesByRune(b, 5)), convey.ShouldEqual, "你")
			convey.So(string(common.TruncateBytesByRune(b, 6)), convey.ShouldEqual, "你好")
		})

		convey.Convey("should handle long string", func() {
			// Generate a long string
			s := strings.Repeat("a", 1000)
			b := []byte(s)
			convey.So(len(common.TruncateBytesByRune(b, 500)), convey.ShouldEqual, 500)
		})
	})
}
