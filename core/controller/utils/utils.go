package utils

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func ParsePageParams(c *gin.Context) (page, perPage int) {
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = c.Query("p")
	}
	page, _ = strconv.Atoi(pageStr)
	perPage, _ = strconv.Atoi(c.Query("per_page"))
	return
}
