//nolint:testpackage
package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseCommonParamsUsesIncludeDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "include_detail=true",
		},
	}

	params := parseCommonParams(c)
	if !params.includeDetail {
		t.Fatal("expected include_detail=true to enable detailed log loading")
	}
}

func TestParseCommonParamsIgnoresLegacyWithBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL: &url.URL{
			RawQuery: "with_body=true",
		},
	}

	params := parseCommonParams(c)
	if params.includeDetail {
		t.Fatal("expected legacy with_body to be ignored")
	}
}
