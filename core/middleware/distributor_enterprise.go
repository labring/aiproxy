package middleware

import "github.com/labring/aiproxy/core/model"

// EnterpriseQuotaCheck is set by enterprise build tag to add progressive quota tier checking.
// Returns: effectiveModel, rpmMultiplier, tpmMultiplier, blocked
var EnterpriseQuotaCheck func(group model.GroupCache, token model.TokenCache, requestModel string) (string, float64, float64, bool)
