package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/network"
	"github.com/labring/aiproxy/core/model"
)

func MCPAuth(c *gin.Context) {
	log := common.GetLogger(c)

	key := c.Request.Header.Get("Authorization")
	if key == "" {
		key, _ = c.GetQuery("key")
	}

	key = strings.TrimPrefix(
		strings.TrimPrefix(key, "Bearer "),
		"sk-",
	)

	var (
		token            model.TokenCache
		useInternalToken bool
	)

	if config.AdminKey != "" && config.AdminKey == key ||
		config.InternalToken != "" && config.InternalToken == key {
		token = model.TokenCache{
			Key: key,
		}
		useInternalToken = true
	} else {
		tokenCache, err := model.GetAndValidateToken(key)
		if err != nil {
			AbortLogWithMessage(c, http.StatusUnauthorized, err.Error())
			return
		}

		token = *tokenCache
	}

	SetLogTokenFields(log.Data, token, useInternalToken)

	if len(token.Subnets) > 0 {
		if ok, err := network.IsIPInSubnets(c.ClientIP(), token.Subnets); err != nil {
			AbortLogWithMessage(c, http.StatusInternalServerError, err.Error())
			return
		} else if !ok {
			AbortLogWithMessage(
				c,
				http.StatusForbidden,
				fmt.Sprintf(
					"token (%s[%d]) can only be used in the specified subnets: %v, current ip: %s",
					token.Name,
					token.ID,
					token.Subnets,
					c.ClientIP(),
				),
			)

			return
		}
	}

	var group model.GroupCache
	if useInternalToken {
		group = model.GroupCache{
			Status: model.GroupStatusInternal,
		}
	} else {
		groupCache, err := model.CacheGetGroup(token.Group)
		if err != nil {
			AbortLogWithMessage(
				c,
				http.StatusInternalServerError,
				fmt.Sprintf("failed to get group: %v", err),
			)

			return
		}

		group = *groupCache
	}

	SetLogGroupFields(log.Data, group)

	if group.Status != model.GroupStatusEnabled && group.Status != model.GroupStatusInternal {
		AbortLogWithMessage(c, http.StatusForbidden, "group is disabled")
		return
	}

	c.Set(Group, group)
	c.Set(Token, token)

	c.Next()
}
