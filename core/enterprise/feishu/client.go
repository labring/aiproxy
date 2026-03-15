//go:build enterprise

package feishu

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	log "github.com/sirupsen/logrus"
)

var (
	client     *lark.Client
	clientOnce sync.Once
)

// GetAppID returns the Feishu app ID from environment.
func GetAppID() string {
	return os.Getenv("FEISHU_APP_ID")
}

// GetAppSecret returns the Feishu app secret from environment.
func GetAppSecret() string {
	return os.Getenv("FEISHU_APP_SECRET")
}

// GetRedirectURI returns the OAuth redirect URI from environment.
func GetRedirectURI() string {
	return os.Getenv("FEISHU_REDIRECT_URI")
}

// GetFrontendURL returns the frontend base URL for post-auth redirect.
// Defaults to "http://localhost:5173" if not set.
func GetFrontendURL() string {
	if v := os.Getenv("FEISHU_FRONTEND_URL"); v != "" {
		return v
	}

	return "http://localhost:5173"
}

// GetAllowedTenants returns the list of allowed tenant keys from environment.
// Multiple tenants can be specified as comma-separated values.
// If empty, all tenants are allowed (no restriction).
func GetAllowedTenants() []string {
	v := os.Getenv("FEISHU_ALLOWED_TENANTS")
	if v == "" {
		return nil
	}

	var tenants []string
	for _, t := range strings.Split(v, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tenants = append(tenants, t)
		}
	}

	return tenants
}

// IsTenantAllowed checks if the given tenant key is in the allowed list.
// Returns true if:
// - The allowed list is empty (no restriction)
// - The tenant key is in the allowed list
func IsTenantAllowed(tenantKey string) bool {
	allowed := GetAllowedTenants()
	if len(allowed) == 0 {
		return true // No restriction
	}

	for _, t := range allowed {
		if t == tenantKey {
			return true
		}
	}

	return false
}

// GetClient returns the singleton Feishu Lark SDK client.
// The SDK handles tenant_access_token caching internally.
func GetClient() *lark.Client {
	clientOnce.Do(func() {
		appID := GetAppID()
		appSecret := GetAppSecret()

		if appID == "" || appSecret == "" {
			log.Warn("FEISHU_APP_ID or FEISHU_APP_SECRET not set, Feishu integration disabled")
		}

		client = lark.NewClient(appID, appSecret)
	})

	return client
}

// UserInfo holds the essential user info returned by Feishu.
type UserInfo struct {
	OpenID   string
	UnionID  string
	UserID   string
	Name     string
	Email    string
	Avatar   string
	TenantID string
}

// GetUserInfo retrieves user info using a user_access_token obtained from OAuth.
func GetUserInfo(ctx context.Context, userAccessToken string) (*UserInfo, error) {
	c := GetClient()

	resp, err := c.Authen.UserInfo.Get(ctx, larkcore.WithUserAccessToken(userAccessToken))
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		log.Errorf("feishu get user info failed: code=%d, msg=%s", resp.Code, resp.Msg)
		return nil, resp.CodeError
	}

	info := &UserInfo{}
	if resp.Data.OpenId != nil {
		info.OpenID = *resp.Data.OpenId
	}

	if resp.Data.UnionId != nil {
		info.UnionID = *resp.Data.UnionId
	}

	if resp.Data.UserId != nil {
		info.UserID = *resp.Data.UserId
	}

	if resp.Data.Name != nil {
		info.Name = *resp.Data.Name
	}

	if resp.Data.Email != nil {
		info.Email = *resp.Data.Email
	}

	if resp.Data.AvatarUrl != nil {
		info.Avatar = *resp.Data.AvatarUrl
	}

	if resp.Data.TenantKey != nil {
		info.TenantID = *resp.Data.TenantKey
	}

	return info, nil
}

// DepartmentInfo holds a Feishu department's core fields.
type DepartmentInfo struct {
	DepartmentID     string
	OpenDepartmentID string
	ParentID         string
	Name             string
	MemberCount      int
	Order            int
}

// ListDepartments fetches all child departments under the given parentID.
// Pass "0" for the root department.
func ListDepartments(ctx context.Context, parentID string) ([]*DepartmentInfo, error) {
	c := GetClient()

	var departments []*DepartmentInfo

	var pageToken *string

	for {
		reqBuilder := larkcontact.NewChildrenDepartmentReqBuilder().
			DepartmentId(parentID).
			PageSize(50)

		if pageToken != nil {
			reqBuilder.PageToken(*pageToken)
		}

		resp, err := c.Contact.Department.Children(ctx, reqBuilder.Build())
		if err != nil {
			return nil, err
		}

		if !resp.Success() {
			log.Errorf("feishu list departments failed: code=%d, msg=%s", resp.Code, resp.Msg)
			return nil, resp.CodeError
		}

		for _, dept := range resp.Data.Items {
			d := &DepartmentInfo{}
			if dept.DepartmentId != nil {
				d.DepartmentID = *dept.DepartmentId
			}

			if dept.OpenDepartmentId != nil {
				d.OpenDepartmentID = *dept.OpenDepartmentId
			}

			if dept.ParentDepartmentId != nil {
				d.ParentID = *dept.ParentDepartmentId
			}

			if dept.Name != nil {
				d.Name = *dept.Name
			}

			if dept.MemberCount != nil {
				d.MemberCount = int(*dept.MemberCount)
			}

			if dept.Order != nil {
				if v, err := strconv.Atoi(*dept.Order); err == nil {
					d.Order = v
				}
			}

			departments = append(departments, d)
		}

		if resp.Data.HasMore == nil || !*resp.Data.HasMore {
			break
		}

		pageToken = resp.Data.PageToken
	}

	return departments, nil
}

// DepartmentUserInfo holds a user's core fields from the contact API.
type DepartmentUserInfo struct {
	OpenID       string
	UnionID      string
	UserID       string
	Name         string
	Email        string
	Avatar       string
	DepartmentID string
}

// ListDepartmentUsers fetches all users in the given department.
func ListDepartmentUsers(ctx context.Context, departmentID string) ([]*DepartmentUserInfo, error) {
	c := GetClient()

	var users []*DepartmentUserInfo

	var pageToken *string

	for {
		reqBuilder := larkcontact.NewFindByDepartmentUserReqBuilder().
			DepartmentId(departmentID).
			PageSize(50)

		if pageToken != nil {
			reqBuilder.PageToken(*pageToken)
		}

		resp, err := c.Contact.User.FindByDepartment(ctx, reqBuilder.Build())
		if err != nil {
			return nil, err
		}

		if !resp.Success() {
			log.Errorf("feishu list department users failed: code=%d, msg=%s", resp.Code, resp.Msg)
			return nil, resp.CodeError
		}

		for _, u := range resp.Data.Items {
			user := &DepartmentUserInfo{}
			if u.OpenId != nil {
				user.OpenID = *u.OpenId
			}

			if u.UnionId != nil {
				user.UnionID = *u.UnionId
			}

			if u.UserId != nil {
				user.UserID = *u.UserId
			}

			if u.Name != nil {
				user.Name = *u.Name
			}

			if u.Email != nil {
				user.Email = *u.Email
			}

			if u.Avatar != nil && u.Avatar.AvatarOrigin != nil {
				user.Avatar = *u.Avatar.AvatarOrigin
			}

			if len(u.DepartmentIds) > 0 {
				user.DepartmentID = u.DepartmentIds[0]
			}

			users = append(users, user)
		}

		if resp.Data.HasMore == nil || !*resp.Data.HasMore {
			break
		}

		pageToken = resp.Data.PageToken
	}

	return users, nil
}
