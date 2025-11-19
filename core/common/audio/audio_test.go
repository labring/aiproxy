package audio_test

import (
	"testing"

	"github.com/labring/aiproxy/core/common/audio"
	"github.com/smartystreets/goconvey/convey"
)

func TestParseTimeFromFfmpegOutput(t *testing.T) {
	convey.Convey("parseTimeFromFfmpegOutput", t, func() {
		convey.Convey("should parse valid duration", func() {
			output := "size=N/A time=00:00:05.52 bitrate=N/A speed= 785x"
			duration, err := audio.ParseTimeFromFfmpegOutput(output)
			convey.So(err, convey.ShouldBeNil)
			convey.So(duration, convey.ShouldAlmostEqual, 5.52)
		})

		convey.Convey("should parse longer duration", func() {
			output := "frame=  100 fps=0.0 q=-0.0 size=   123kB time=01:02:03.45 bitrate=  10.0kbits/s speed=  10x"
			duration, err := audio.ParseTimeFromFfmpegOutput(output)
			convey.So(err, convey.ShouldBeNil)
			// 1*3600 + 2*60 + 3.45 = 3600 + 120 + 3.45 = 3723.45
			convey.So(duration, convey.ShouldAlmostEqual, 3723.45)
		})

		convey.Convey("should use last match", func() {
			output := "time=00:00:01.00\n... time=00:00:02.00"
			duration, err := audio.ParseTimeFromFfmpegOutput(output)
			convey.So(err, convey.ShouldBeNil)
			convey.So(duration, convey.ShouldAlmostEqual, 2.0)
		})

		convey.Convey("should return error for no time match", func() {
			output := "invalid output"
			_, err := audio.ParseTimeFromFfmpegOutput(output)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err, convey.ShouldEqual, audio.ErrAudioDurationNAN)
		})
	})
}
