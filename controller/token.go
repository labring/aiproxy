package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/common/network"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/model"
)

// TokenResponse represents the response structure for token endpoints
type TokenResponse struct {
	*model.Token
	AccessedAt time.Time `json:"accessed_at"`
}

func (t *TokenResponse) MarshalJSON() ([]byte, error) {
	type Alias TokenResponse
	return sonic.Marshal(&struct {
		*Alias
		CreatedAt  int64 `json:"created_at"`
		ExpiredAt  int64 `json:"expired_at"`
		AccessedAt int64 `json:"accessed_at"`
	}{
		Alias:      (*Alias)(t),
		CreatedAt:  t.CreatedAt.UnixMilli(),
		ExpiredAt:  t.ExpiredAt.UnixMilli(),
		AccessedAt: t.AccessedAt.UnixMilli(),
	})
}

type (
	AddTokenRequest struct {
		Name      string   `json:"name"`
		Subnets   []string `json:"subnets"`
		Models    []string `json:"models"`
		ExpiredAt int64    `json:"expiredAt"`
		Quota     float64  `json:"quota"`
	}

	UpdateTokenStatusRequest struct {
		Status int `json:"status"`
	}

	UpdateTokenNameRequest struct {
		Name string `json:"name"`
	}
)

func (at *AddTokenRequest) ToToken() *model.Token {
	var expiredAt time.Time
	if at.ExpiredAt > 0 {
		expiredAt = time.UnixMilli(at.ExpiredAt)
	}
	return &model.Token{
		Name:      model.EmptyNullString(at.Name),
		Subnets:   at.Subnets,
		Models:    at.Models,
		ExpiredAt: expiredAt,
		Quota:     at.Quota,
	}
}

func validateToken(token AddTokenRequest) error {
	if token.Name == "" {
		return errors.New("token name cannot be empty")
	}
	if len(token.Name) > 30 {
		return errors.New("token name is too long")
	}
	if err := network.IsValidSubnets(token.Subnets); err != nil {
		return fmt.Errorf("invalid subnet: %w", err)
	}
	return nil
}

func validateTokenUpdate(token AddTokenRequest) error {
	if err := network.IsValidSubnets(token.Subnets); err != nil {
		return fmt.Errorf("invalid subnet: %w", err)
	}
	return nil
}

func buildTokenResponse(token *model.Token) *TokenResponse {
	lastRequestAt, _ := model.GetTokenLastRequestTime(token.ID)
	return &TokenResponse{
		Token:      token,
		AccessedAt: lastRequestAt,
	}
}

func buildTokenResponses(tokens []*model.Token) []*TokenResponse {
	responses := make([]*TokenResponse, len(tokens))
	for i, token := range tokens {
		responses[i] = buildTokenResponse(token)
	}
	return responses
}

// GetTokens godoc
//
//	@Summary		Get all tokens
//	@Description	Returns a paginated list of all tokens
//	@Tags			tokens
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			page		query		int	false	"Page number"
//	@Param			per_page	query		int	false	"Items per page"
//	@Success		200			{object}	middleware.APIResponse{data=map[string]any{tokens=[]TokenResponse,total=int}}
//	@Router			/api/tokens [get]
func GetTokens(c *gin.Context) {
	page, perPage := parsePageParams(c)
	group := c.Query("group")
	order := c.Query("order")
	status, _ := strconv.Atoi(c.Query("status"))

	tokens, total, err := model.GetTokens(group, page, perPage, order, status)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"tokens": buildTokenResponses(tokens),
		"total":  total,
	})
}

// GetGroupTokens godoc
//
//	@Summary		Get all tokens for a specific group
//	@Description	Returns a paginated list of all tokens for a specific group
//	@Tags			tokens
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse{data=map[string]any{tokens=[]TokenResponse,total=int}}
//	@Router			/api/tokens/{group} [get]
func GetGroupTokens(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusOK, "group is required")
		return
	}

	page, perPage := parsePageParams(c)
	order := c.Query("order")
	status, _ := strconv.Atoi(c.Query("status"))

	tokens, total, err := model.GetTokens(group, page, perPage, order, status)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"tokens": buildTokenResponses(tokens),
		"total":  total,
	})
}

// SearchTokens godoc
//
//	@Summary		Search tokens
//	@Description	Returns a paginated list of tokens based on search criteria
//	@Tags			tokens
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			keyword	query		string	false	"Keyword"
//	@Param			order	query		string	false	"Order"
//	@Param			name	query		string	false	"Name"
//	@Param			key		query		string	false	"Key"
//	@Param			status	query		int		false	"Status"
//	@Param			group	query		string	false	"Group"
//	@Success		200		{object}	middleware.APIResponse{data=map[string]any{tokens=[]TokenResponse,total=int}}
//	@Router			/api/tokens/search [get]
func SearchTokens(c *gin.Context) {
	page, perPage := parsePageParams(c)
	keyword := c.Query("keyword")
	order := c.Query("order")
	name := c.Query("name")
	key := c.Query("key")
	status, _ := strconv.Atoi(c.Query("status"))
	group := c.Query("group")

	tokens, total, err := model.SearchTokens(group, keyword, page, perPage, order, status, name, key)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"tokens": buildTokenResponses(tokens),
		"total":  total,
	})
}

// SearchGroupTokens godoc
//
//	@Summary		Search tokens for a specific group
//	@Description	Returns a paginated list of tokens for a specific group based on search criteria
//	@Tags			tokens
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse{data=map[string]any{tokens=[]TokenResponse,total=int}}
//	@Router			/api/token/{group}/search [get]
func SearchGroupTokens(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusOK, "group is required")
		return
	}

	page, perPage := parsePageParams(c)
	keyword := c.Query("keyword")
	order := c.Query("order")
	name := c.Query("name")
	key := c.Query("key")
	status, _ := strconv.Atoi(c.Query("status"))

	tokens, total, err := model.SearchTokens(group, keyword, page, perPage, order, status, name, key)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"tokens": buildTokenResponses(tokens),
		"total":  total,
	})
}

// GetToken godoc
//
//	@Summary		Get token by ID
//	@Description	Returns detailed information about a specific token
//	@Tags			tokens
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Token ID"
//	@Success		200	{object}	middleware.APIResponse{data=TokenResponse}
//	@Router			/api/tokens/{id} [get]
func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	token, err := model.GetTokenByID(id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, buildTokenResponse(token))
}

// GetGroupToken godoc
//
//	@Summary		Get token by ID for a specific group
//	@Description	Returns detailed information about a specific token for a specific group
//	@Tags			tokens
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			id		path		int		true	"Token ID"
//	@Success		200		{object}	middleware.APIResponse{data=TokenResponse}
//	@Router			/api/token/{group}/{id} [get]
func GetGroupToken(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusOK, "group is required")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	token, err := model.GetGroupTokenByID(group, id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, buildTokenResponse(token))
}

// AddGroupToken godoc
//
//	@Summary		Add group token
//	@Description	Adds a new token to a specific group
//	@Tags			token
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string		true	"Group name"
//	@Param			token	body		model.Token	true	"Token information"
//	@Success		200		{object}	middleware.APIResponse{data=TokenResponse}
//	@Router			/api/token/{group} [post]
func AddGroupToken(c *gin.Context) {
	group := c.Param("group")
	var req AddTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := validateToken(req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, "parameter error: "+err.Error())
		return
	}

	token := req.ToToken()
	token.GroupID = group

	if err := model.InsertToken(token, c.Query("auto_create_group") == "true"); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, &TokenResponse{Token: token})
}

// DeleteToken godoc
//
//	@Summary		Delete token
//	@Description	Deletes a specific token by ID
//	@Tags			tokens
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Token ID"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/tokens/{id} [delete]
func DeleteToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.DeleteTokenByID(id); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteTokens godoc
//
//	@Summary		Delete multiple tokens
//	@Description	Deletes multiple tokens by their IDs
//	@Tags			tokens
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			ids	body		[]int	true	"Token IDs"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/tokens/batch_delete [post]
func DeleteTokens(c *gin.Context) {
	var ids []int
	if err := c.ShouldBindJSON(&ids); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.DeleteTokensByIDs(ids); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGroupToken godoc
//
//	@Summary		Delete group token
//	@Description	Deletes a specific token from a group
//	@Tags			token
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			id		path		int		true	"Token ID"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/token/{group}/{id} [delete]
func DeleteGroupToken(c *gin.Context) {
	group := c.Param("group")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.DeleteGroupTokenByID(group, id); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGroupTokens godoc
//
//	@Summary		Delete group tokens
//	@Description	Deletes multiple tokens from a specific group
//	@Tags			token
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			ids		body		[]int	true	"Token IDs"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/token/{group}/batch_delete [post]
func DeleteGroupTokens(c *gin.Context) {
	group := c.Param("group")
	var ids []int
	if err := c.ShouldBindJSON(&ids); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.DeleteGroupTokensByIDs(group, ids); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// UpdateToken godoc
//
//	@Summary		Update token
//	@Description	Updates an existing token's information
//	@Tags			tokens
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int			true	"Token ID"
//	@Param			token	body		model.Token	true	"Updated token information"
//	@Success		200		{object}	middleware.APIResponse{data=TokenResponse}
//	@Router			/api/tokens/{id} [put]
func UpdateToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	var req AddTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := validateTokenUpdate(req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, "parameter error: "+err.Error())
		return
	}

	token := req.ToToken()

	if err := model.UpdateToken(id, token); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, &TokenResponse{Token: token})
}

// UpdateGroupToken godoc
//
//	@Summary		Update group token
//	@Description	Updates an existing token in a specific group
//	@Tags			token
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string		true	"Group name"
//	@Param			id		path		int			true	"Token ID"
//	@Param			token	body		model.Token	true	"Updated token information"
//	@Success		200		{object}	middleware.APIResponse{data=TokenResponse}
//	@Router			/api/token/{group}/{id} [put]
func UpdateGroupToken(c *gin.Context) {
	group := c.Param("group")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	var req AddTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := validateTokenUpdate(req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, "parameter error: "+err.Error())
		return
	}

	token := req.ToToken()

	if err := model.UpdateGroupToken(id, group, token); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, &TokenResponse{Token: token})
}

// UpdateTokenStatus godoc
//
//	@Summary		Update token status
//	@Description	Updates the status of a specific token
//	@Tags			tokens
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int							true	"Token ID"
//	@Param			status	body		UpdateTokenStatusRequest	true	"Status information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/tokens/{id}/status [post]
func UpdateTokenStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	var req UpdateTokenStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.UpdateTokenStatus(id, req.Status); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// UpdateGroupTokenStatus godoc
//
//	@Summary		Update group token status
//	@Description	Updates the status of a token in a specific group
//	@Tags			token
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string						true	"Group name"
//	@Param			id		path		int							true	"Token ID"
//	@Param			status	body		UpdateTokenStatusRequest	true	"Status information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/token/{group}/{id}/status [post]
func UpdateGroupTokenStatus(c *gin.Context) {
	group := c.Param("group")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	var req UpdateTokenStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.UpdateGroupTokenStatus(group, id, req.Status); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// UpdateTokenName godoc
//
//	@Summary		Update token name
//	@Description	Updates the name of a specific token
//	@Tags			tokens
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int						true	"Token ID"
//	@Param			name	body		UpdateTokenNameRequest	true	"Name information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/tokens/{id}/name [post]
func UpdateTokenName(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	var req UpdateTokenNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.UpdateTokenName(id, req.Name); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// UpdateGroupTokenName godoc
//
//	@Summary		Update group token name
//	@Description	Updates the name of a token in a specific group
//	@Tags			token
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string					true	"Group name"
//	@Param			id		path		int						true	"Token ID"
//	@Param			name	body		UpdateTokenNameRequest	true	"Name information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/token/{group}/{id}/name [post]
func UpdateGroupTokenName(c *gin.Context) {
	group := c.Param("group")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	var req UpdateTokenNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	if err := model.UpdateGroupTokenName(group, id, req.Name); err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}
