package model_test

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
)

func TestRequestDetailApplyBodySizeLimits(t *testing.T) {
	detail := &model.RequestDetail{
		RequestBody:  "abcdef",
		ResponseBody: "uvwxyz",
	}

	detail.ApplyBodySizeLimits(4, -1)

	if detail.RequestBody != "a..." {
		t.Fatalf("expected request body to be truncated to a..., got %q", detail.RequestBody)
	}

	if !detail.RequestBodyTruncated {
		t.Fatal("expected request body truncated flag to be true")
	}

	if detail.ResponseBody != "" {
		t.Fatalf("expected response body to be cleared, got %q", detail.ResponseBody)
	}

	if !detail.ResponseBodyTruncated {
		t.Fatal("expected response body truncated flag to be true")
	}
}

func TestRequestDetailApplyBodySizeLimitsZeroKeepsOriginalBody(t *testing.T) {
	detail := &model.RequestDetail{
		RequestBody:  "abcdef",
		ResponseBody: "你好世界",
	}

	detail.ApplyBodySizeLimits(0, 0)

	if detail.RequestBody != "abcdef" {
		t.Fatalf("expected request body to remain unchanged, got %q", detail.RequestBody)
	}

	if detail.RequestBodyTruncated {
		t.Fatal("expected request body truncated flag to remain false")
	}

	if detail.ResponseBody != "你好世界" {
		t.Fatalf("expected response body to remain unchanged, got %q", detail.ResponseBody)
	}

	if detail.ResponseBodyTruncated {
		t.Fatal("expected response body truncated flag to remain false")
	}
}
