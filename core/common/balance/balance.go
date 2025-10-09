package balance

import (
	"context"

	"github.com/labring/aiproxy/core/model"
)

type GroupBalance interface {
	GetGroupRemainBalance(
		ctx context.Context,
		group model.GroupCache,
	) (float64, PostGroupConsumer, error)

	GetGroupQuota(ctx context.Context, group model.GroupCache) (*GroupQuota, error)
}

type PostGroupConsumer interface {
	PostGroupConsume(ctx context.Context, tokenName string, usage float64) (float64, error)
}

type GroupQuota struct {
	Total  float64 `json:"total"`
	Remain float64 `json:"remain"`
}

var (
	mock    GroupBalance = NewMockGroupBalance()
	Default              = mock
)

func MockGetGroupRemainBalance(
	ctx context.Context,
	group model.GroupCache,
) (float64, PostGroupConsumer, error) {
	return mock.GetGroupRemainBalance(ctx, group)
}

func GetGroupRemainBalance(
	ctx context.Context,
	group model.GroupCache,
) (float64, PostGroupConsumer, error) {
	return Default.GetGroupRemainBalance(ctx, group)
}

func GetGroupQuota(ctx context.Context, group model.GroupCache) (*GroupQuota, error) {
	return Default.GetGroupQuota(ctx, group)
}
