package model_test

import (
	"errors"
	"go/constant"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/smartystreets/goconvey/convey"
	"gorm.io/gorm"
)

func TestString2Int(t *testing.T) {
	convey.Convey("String2Int", t, func() {
		convey.Convey("should convert valid string", func() {
			convey.So(model.String2Int("123"), convey.ShouldEqual, 123)
		})
		convey.Convey("should return 0 for empty string", func() {
			convey.So(model.String2Int(""), convey.ShouldEqual, 0)
		})
		convey.Convey("should return 0 for invalid string", func() {
			convey.So(model.String2Int("abc"), convey.ShouldEqual, 0)
		})
	})
}

func TestToLimitOffset(t *testing.T) {
	convey.Convey("toLimitOffset", t, func() {
		convey.Convey("should calculate correct offset", func() {
			limit, offset := model.ToLimitOffset(1, 10)
			convey.So(limit, convey.ShouldEqual, 10)
			convey.So(offset, convey.ShouldEqual, 0)

			limit, offset = model.ToLimitOffset(2, 10)
			convey.So(limit, convey.ShouldEqual, 10)
			convey.So(offset, convey.ShouldEqual, 10)
		})

		convey.Convey("should handle page < 1", func() {
			_, offset := model.ToLimitOffset(0, 10)
			convey.So(offset, convey.ShouldEqual, 0)
		})

		convey.Convey("should clamp perPage", func() {
			// Default 10 if <= 0
			limit, _ := model.ToLimitOffset(1, 0)
			convey.So(limit, convey.ShouldEqual, 10)

			// Max 100
			limit, _ = model.ToLimitOffset(1, 101)
			convey.So(limit, convey.ShouldEqual, 100)
		})
	})
}

func TestErrors(t *testing.T) {
	convey.Convey("Errors", t, func() {
		convey.Convey("NotFoundError", func() {
			err := model.NotFoundError("user")
			convey.So(errors.Is(err, gorm.ErrRecordNotFound), convey.ShouldBeTrue)
			convey.So(err.Error(), convey.ShouldContainSubstring, "user")
		})

		convey.Convey("HandleNotFound", func() {
			// Should wrap ErrRecordNotFound
			err := model.HandleNotFound(gorm.ErrRecordNotFound, "user")
			convey.So(errors.Is(err, gorm.ErrRecordNotFound), convey.ShouldBeTrue)
			convey.So(err.Error(), convey.ShouldContainSubstring, "user")

			// Should pass through other errors
			otherErr := errors.New("other error")
			err = model.HandleNotFound(otherErr, "user")
			convey.So(err, convey.ShouldEqual, otherErr)

			// Should return nil for nil
			err = model.HandleNotFound(nil, "user")
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("IgnoreNotFound", func() {
			// Should ignore ErrRecordNotFound
			err := model.IgnoreNotFound(gorm.ErrRecordNotFound)
			convey.So(err, convey.ShouldBeNil)

			// Should pass through other errors
			otherErr := errors.New("other error")
			err = model.IgnoreNotFound(otherErr)
			convey.So(err, convey.ShouldEqual, otherErr)

			// Should return nil for nil
			err = model.IgnoreNotFound(nil)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("HandleUpdateResult", func() {
			// Error case
			res := &gorm.DB{Error: errors.New("db error")}
			err := model.HandleUpdateResult(res, "user")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "db error")

			// RowsAffected == 0 case (should be NotFoundError)
			res = &gorm.DB{Error: nil, RowsAffected: 0}
			err = model.HandleUpdateResult(res, "user")
			convey.So(errors.Is(err, gorm.ErrRecordNotFound), convey.ShouldBeTrue)
			convey.So(err.Error(), convey.ShouldContainSubstring, "user")

			// Success case
			res = &gorm.DB{Error: nil, RowsAffected: 1}
			err = model.HandleUpdateResult(res, "user")
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestZeroNullTypes(t *testing.T) {
	convey.Convey("ZeroNullInt64", t, func() {
		convey.Convey("Value", func() {
			var z model.ZeroNullInt64 = 0

			v, _ := z.Value()
			convey.So(v, convey.ShouldBeNil)

			z = 10
			v, _ = z.Value()
			convey.So(v, convey.ShouldEqual, 10)
		})

		convey.Convey("Scan", func() {
			var z model.ZeroNullInt64

			// Nil
			convey.So(z.Scan(nil), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 0)

			// Int
			convey.So(z.Scan(int(123)), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 123)

			// Int64
			convey.So(z.Scan(int64(456)), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 456)

			// String
			convey.So(z.Scan("789"), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 789)

			// Invalid type
			err := z.Scan(true)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "unsupported type")

			// Invalid string
			err = z.Scan("abc")
			convey.So(err, convey.ShouldNotBeNil)
		})
	})

	convey.Convey("ZeroNullFloat64", t, func() {
		convey.Convey("Value", func() {
			var z model.ZeroNullFloat64 = 0

			v, _ := z.Value()
			convey.So(v, convey.ShouldBeNil)

			z = 10.5
			v, _ = z.Value()
			convey.So(v, convey.ShouldEqual, 10.5)
		})

		convey.Convey("Scan", func() {
			var z model.ZeroNullFloat64

			// Nil
			convey.So(z.Scan(nil), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 0)

			// Float64
			convey.So(z.Scan(10.5), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 10.5)

			// String
			convey.So(z.Scan("20.5"), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 20.5)

			// Int (implicit conversion in switch)
			convey.So(z.Scan(int(30)), convey.ShouldBeNil)
			convey.So(z, convey.ShouldEqual, 30.0)

			// Invalid type
			err := z.Scan(true)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "unsupported type")

			// Invalid string
			err = z.Scan("abc")
			convey.So(err, convey.ShouldNotBeNil)
		})
	})

	convey.Convey("EmptyNullString", t, func() {
		convey.Convey("Value", func() {
			var s model.EmptyNullString = ""

			v, _ := s.Value()
			convey.So(v, convey.ShouldBeNil)

			s = "test"
			v, _ = s.Value()
			convey.So(v, convey.ShouldEqual, "test")
		})

		convey.Convey("Scan", func() {
			var s model.EmptyNullString

			convey.So(s.Scan(nil), convey.ShouldBeNil)
			convey.So(string(s), convey.ShouldEqual, "")

			convey.So(s.Scan("test"), convey.ShouldBeNil)
			convey.So(string(s), convey.ShouldEqual, "test")

			convey.So(s.Scan([]byte("bytes")), convey.ShouldBeNil)
			convey.So(string(s), convey.ShouldEqual, "bytes")

			// Invalid type
			err := s.Scan(123)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "unsupported type")
		})

		convey.Convey(constant.String.String(), func() {
			var s model.EmptyNullString = "test"
			convey.So(s.String(), convey.ShouldEqual, "test")
		})
	})
}
