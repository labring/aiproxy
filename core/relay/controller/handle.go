package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
)

// HandleResult contains all the information needed for consumption recording
type HandleResult struct {
	Error      adaptor.Error
	Usage      model.Usage
	UpstreamID string
	BodyDetail *BodyDetail
}

func Handle(
	adaptor adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
	opts ...BodyDetailOption,
) *HandleResult {
	log := common.GetLogger(c)

	result, detail, respErr := DoHelper(adaptor, c, meta, store, opts...)
	if respErr != nil {
		if detail != nil && config.DebugEnabled &&
			(detail.RequestBody != "" || detail.ResponseBody != "") {
			log.Errorf(
				"handle failed: %+v\nrequest detail:\n%s\nresponse detail:\n%s",
				respErr,
				detail.RequestBody,
				detail.ResponseBody,
			)
		} else {
			log.Errorf("handle failed: %+v", respErr)
		}

		return &HandleResult{
			Error:      respErr,
			Usage:      result.Usage,
			UpstreamID: result.UpstreamID,
			BodyDetail: detail,
		}
	}

	return &HandleResult{
		Usage:      result.Usage,
		UpstreamID: result.UpstreamID,
		BodyDetail: detail,
	}
}
