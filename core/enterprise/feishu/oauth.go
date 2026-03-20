//go:build enterprise

package feishu

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	enterprisemodels "github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

const feishuOAuthAuthorizeURL = "https://open.feishu.cn/open-apis/authen/v1/authorize"

// HandleLogin redirects the user to Feishu's OAuth authorization page.
func HandleLogin(c *gin.Context) {
	appID := GetAppID()
	redirectURI := GetRedirectURI()

	if appID == "" || redirectURI == "" {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "feishu OAuth is not configured")
		return
	}

	state := c.Query("state")

	params := url.Values{}
	params.Set("app_id", appID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	if state != "" {
		params.Set("state", state)
	}

	authURL := feishuOAuthAuthorizeURL + "?" + params.Encode()
	c.Redirect(http.StatusFound, authURL)
}

// HandleCallback processes the Feishu OAuth callback.
// It exchanges the authorization code for a user_access_token,
// fetches user info, upserts the FeishuUser, ensures a Group and Token exist,
// and returns the token key.
func HandleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "missing authorization code")
		return
	}

	ctx := c.Request.Context()

	// Exchange code for user_access_token
	client := GetClient()

	tokenReq := larkauthen.NewCreateAccessTokenReqBuilder().
		Body(larkauthen.NewCreateAccessTokenReqBodyBuilder().
			GrantType("authorization_code").
			Code(code).
			Build()).
		Build()

	tokenResp, err := client.Authen.AccessToken.Create(ctx, tokenReq)
	if err != nil {
		log.Errorf("feishu exchange token failed: %v", err)
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to exchange authorization code")

		return
	}

	if !tokenResp.Success() {
		log.Errorf("feishu exchange token error: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
		middleware.ErrorResponse(c, http.StatusInternalServerError, "feishu token exchange failed")

		return
	}

	if tokenResp.Data.AccessToken == nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "feishu returned empty access token")
		return
	}

	userAccessToken := *tokenResp.Data.AccessToken

	// Get user info
	userInfo, err := GetUserInfo(ctx, userAccessToken)
	if err != nil {
		log.Errorf("feishu get user info failed: %v", err)
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to get user info from feishu")

		return
	}

	if userInfo.OpenID == "" {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "feishu returned empty open_id")
		return
	}

	// Validate tenant whitelist
	// Check using database-backed whitelist (imported from parent enterprise package)
	isTenantAllowed := checkTenantAccess(userInfo.TenantID)

	if !isTenantAllowed {
		log.Warnf("feishu login rejected: tenant %q not in allowed list, user: %s (%s)",
			userInfo.TenantID, userInfo.Name, userInfo.OpenID)

		// Record the rejected login attempt for admin review
		recordRejectedTenant(userInfo.TenantID, userInfo.Name, userInfo.Email)

		// Check if this is a browser request or API request
		accept := c.GetHeader("Accept")
		errorMsg := fmt.Sprintf("Your organization (Tenant ID: %s) is not authorized to access this service. Please contact the administrator to add this tenant ID to the whitelist.", userInfo.TenantID)
		if c.GetHeader("X-Requested-With") != "" || strings.Contains(accept, "application/json") {
			middleware.ErrorResponse(c, http.StatusForbidden, errorMsg)
		} else {
			// Browser flow: redirect to frontend with error
			frontendURL := GetFrontendURL()
			params := url.Values{}
			params.Set("error", "unauthorized_tenant")
			params.Set("message", errorMsg)
			params.Set("tenant_id", userInfo.TenantID)
			redirectURL := fmt.Sprintf("%s/feishu/callback?%s", frontendURL, params.Encode())
			c.Redirect(http.StatusFound, redirectURL)
		}

		return
	}

	// Upsert FeishuUser record
	groupID := fmt.Sprintf("feishu_%s", userInfo.OpenID)
	feishuUser := enterprisemodels.FeishuUser{
		OpenID:   userInfo.OpenID,
		UnionID:  userInfo.UnionID,
		UserID:   userInfo.UserID,
		TenantID: userInfo.TenantID,
		Name:     userInfo.Name,
		Email:    userInfo.Email,
		Avatar:   userInfo.Avatar,
		GroupID:  groupID,
		Status:   1,
	}

	result := model.DB.
		Where("open_id = ?", userInfo.OpenID).
		Assign(enterprisemodels.FeishuUser{
			UnionID:  userInfo.UnionID,
			UserID:   userInfo.UserID,
			TenantID: userInfo.TenantID,
			Name:     userInfo.Name,
			Email:    userInfo.Email,
			Avatar:   userInfo.Avatar,
			GroupID:  groupID,
			Status:   1,
		}).
		FirstOrCreate(&feishuUser)
	if result.Error != nil {
		log.Errorf("feishu upsert user failed: %v", result.Error)
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to save user record")

		return
	}

	// Create Group if not exists (pattern from model.InsertToken with autoCreateGroup)
	group := &model.Group{
		ID: groupID,
	}

	if err := model.OnConflictDoNothing().Create(group).Error; err != nil {
		log.Errorf("feishu create group failed: %v", err)
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to create group")

		return
	}

	// Find or create Token for the user
	tokenName := userInfo.Name
	if tokenName == "" {
		tokenName = userInfo.OpenID
	}

	// Truncate token name to 32 chars (model constraint)
	if len(tokenName) > 32 {
		tokenName = tokenName[:32]
	}

	token := &model.Token{
		GroupID: groupID,
		Name:    model.EmptyNullString(tokenName),
		Status:  model.TokenStatusEnabled,
	}

	if err := model.InsertToken(token, false, true); err != nil {
		log.Errorf("feishu create token failed: %v", err)
		middleware.ErrorResponse(c, http.StatusInternalServerError, "failed to create token")

		return
	}

	// Update feishu_user with token_id if needed
	if feishuUser.TokenID != token.ID && token.ID != 0 {
		model.DB.Model(&feishuUser).Update("token_id", token.ID)
	}

	// If the request comes from the frontend API call (has explicit
	// "application/json" in Accept header, not just wildcard */*),
	// return JSON. Otherwise redirect to the frontend callback page.
	accept := c.GetHeader("Accept")
	if c.GetHeader("X-Requested-With") != "" ||
		strings.Contains(accept, "application/json") {
		middleware.SuccessResponse(c, gin.H{
			"token_key": token.Key,
			"user": gin.H{
				"open_id": userInfo.OpenID,
				"name":    userInfo.Name,
				"email":   userInfo.Email,
				"avatar":  userInfo.Avatar,
			},
		})

		return
	}

	// Browser redirect: pass auth data to frontend via URL params
	frontendURL := GetFrontendURL()
	params := url.Values{}
	params.Set("token_key", token.Key)
	params.Set("open_id", userInfo.OpenID)
	params.Set("name", userInfo.Name)
	params.Set("avatar", userInfo.Avatar)
	if userInfo.Email != "" {
		params.Set("email", userInfo.Email)
	}

	redirectURL := fmt.Sprintf("%s/feishu/callback?%s", frontendURL, params.Encode())
	c.Redirect(http.StatusFound, redirectURL)
}

// checkTenantAccess checks if a tenant is allowed using database or env config.
func checkTenantAccess(tenantKey string) bool {
	// Get config from database
	var config enterprisemodels.TenantWhitelistConfig
	err := model.DB.FirstOrCreate(&config, enterprisemodels.TenantWhitelistConfig{ID: 1}).Error
	if err != nil {
		// Fallback to environment variable
		return IsTenantAllowedByEnv(tenantKey)
	}

	// Use environment variable if env_override is enabled
	if config.EnvOverride {
		return IsTenantAllowedByEnv(tenantKey)
	}

	// Wildcard mode: allow all
	if config.WildcardMode {
		return true
	}

	// Check database whitelist
	var count int64
	model.DB.Model(&enterprisemodels.TenantWhitelist{}).
		Where("tenant_id = ?", tenantKey).
		Count(&count)

	if count > 0 {
		return true
	}

	// If database has records but tenant not found, deny
	var totalCount int64
	model.DB.Model(&enterprisemodels.TenantWhitelist{}).Count(&totalCount)
	if totalCount > 0 {
		return false
	}

	// No database records, fallback to environment variable
	return IsTenantAllowedByEnv(tenantKey)
}

// recordRejectedTenant upserts a rejected login record for the given tenant.
// If the tenant already has a record, it increments the attempt count and updates user info.
// Uses SELECT+UPDATE pattern for SQLite/PostgreSQL compatibility (race is benign here —
// concurrent rejected logins from the same tenant are rare, and worst case loses a count).
func recordRejectedTenant(tenantID, userName, userEmail string) {
	var existing enterprisemodels.RejectedTenantLogin
	result := model.DB.Where("tenant_id = ?", tenantID).First(&existing)

	if result.Error != nil {
		if err := model.DB.Create(&enterprisemodels.RejectedTenantLogin{
			TenantID:      tenantID,
			UserName:      userName,
			UserEmail:     userEmail,
			AttemptCount:  1,
			LastAttemptAt: time.Now(),
		}).Error; err != nil {
			log.Errorf("failed to record rejected tenant login: %v", err)
		}

		return
	}

	if err := model.DB.Model(&existing).Updates(map[string]interface{}{
		"user_name":       userName,
		"user_email":      userEmail,
		"attempt_count":   gorm.Expr("attempt_count + 1"),
		"last_attempt_at": time.Now(),
	}).Error; err != nil {
		log.Errorf("failed to update rejected tenant login: %v", err)
	}
}
