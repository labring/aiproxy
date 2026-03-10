package controller

import (
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
)

func GetRequestServiceTier(c *gin.Context) (string, error) {
	node, err := common.UnmarshalRequest2NodeReusable(c.Request, "service_tier")
	if err != nil {
		return "", err
	}

	if node.TypeSafe() == ast.V_NULL {
		return "", nil
	}

	return node.String()
}
