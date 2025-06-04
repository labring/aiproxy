package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
)

func updateChannelBalance(channel *model.Channel) (float64, error) {
	adaptorI, ok := adaptors.GetAdaptor(channel.Type)
	if !ok {
		return 0, fmt.Errorf(
			"invalid channel type: %d, channel: %s (id: %d)",
			channel.Type,
			channel.Name,
			channel.ID,
		)
	}
	if getBalance, ok := adaptorI.(adaptor.Balancer); ok {
		balance, err := getBalance.GetBalance(channel)
		if err != nil && !errors.Is(err, adaptor.ErrGetBalanceNotImplemented) {
			return 0, fmt.Errorf(
				"failed to get channel %s (type: %d, id: %d) balance: %s",
				channel.Name,
				channel.Type,
				channel.ID,
				err.Error(),
			)
		}
		if err := channel.UpdateBalance(balance); err != nil {
			return 0, fmt.Errorf(
				"failed to update channel %s (type: %d, id: %d) balance: %s",
				channel.Name,
				channel.Type,
				channel.ID,
				err.Error(),
			)
		}
		if !errors.Is(err, adaptor.ErrGetBalanceNotImplemented) &&
			balance < channel.GetBalanceThreshold() {
			return 0, fmt.Errorf(
				"channel %s (type: %d, id: %d) balance is less than threshold: %f",
				channel.Name,
				channel.Type,
				channel.ID,
				balance,
			)
		}
		return balance, nil
	}
	return 0, nil
}

// UpdateChannelBalance godoc
//
//	@Summary		Update channel balance
//	@Description	Updates the balance for a single channel
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		200	{object}	middleware.APIResponse{data=float64}
//	@Router			/api/channel/{id}/balance [get]
func UpdateChannelBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}
	channel, err := model.GetChannelByID(id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}
	balance, err := updateChannelBalance(channel)
	if err != nil {
		notify.Error(
			fmt.Sprintf(
				"check channel %s (type: %d, id: %d) balance error",
				channel.Name,
				channel.Type,
				channel.ID,
			),
			err.Error(),
		)
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}
	middleware.SuccessResponse(c, balance)
}

func updateAllChannelsBalance() error {
	channels, err := model.GetAllChannels()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, channel := range channels {
		if !channel.EnabledAutoBalanceCheck {
			continue
		}
		wg.Add(1)
		semaphore <- struct{}{}
		go func(ch *model.Channel) {
			defer wg.Done()
			defer func() { <-semaphore }()
			_, err := updateChannelBalance(ch)
			if err != nil {
				notify.Error(
					fmt.Sprintf(
						"check channel %s (type: %d, id: %d) balance error",
						ch.Name,
						ch.Type,
						ch.ID,
					),
					err.Error(),
				)
			}
		}(channel)
	}

	wg.Wait()
	return nil
}

// UpdateAllChannelsBalance godoc
//
//	@Summary		Update all channels balance
//	@Description	Updates the balance for all channels
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/channels/balance [get]
func UpdateAllChannelsBalance(c *gin.Context) {
	err := updateAllChannelsBalance()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

func UpdateChannelsBalance(frequency time.Duration) {
	for {
		time.Sleep(frequency)
		_ = updateAllChannelsBalance()
	}
}
