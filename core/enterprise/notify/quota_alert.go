//go:build enterprise

package notify

import "fmt"

// QuotaAlertConfig controls quota alert notification behavior.
type QuotaAlertConfig struct {
	// Enabled controls whether quota alert notifications are sent via webhook.
	Enabled bool
	// P2PEnabled controls whether quota alerts are also sent as P2P messages to users.
	P2PEnabled bool
}

// BuildQuotaTierChangeMessage constructs notification content for a quota tier change.
// It returns a title and message suitable for both webhook and P2P notifications.
func BuildQuotaTierChangeMessage(
	groupID, tokenName string,
	oldTier, newTier int,
	usageRatio float64,
) (title, message string) {
	title = fmt.Sprintf("Quota Tier Change: %s", groupID)

	message = fmt.Sprintf(
		"**Group**: %s\n"+
			"**Token**: %s\n"+
			"**Tier Change**: Tier %d → Tier %d\n"+
			"**Current Usage**: %.1f%%\n\n"+
			"The quota tier has changed. Higher tiers may have reduced rate limits or restricted model access. "+
			"Please review your usage to avoid service disruption.",
		groupID,
		tokenName,
		oldTier,
		newTier,
		usageRatio*100,
	)

	return title, message
}

// BuildQuotaExhaustedMessage constructs notification content for when quota is fully exhausted.
// It returns a title and message suitable for both webhook and P2P notifications.
func BuildQuotaExhaustedMessage(groupID, tokenName string) (title, message string) {
	title = fmt.Sprintf("Quota Exhausted: %s", groupID)

	message = fmt.Sprintf(
		"**Group**: %s\n"+
			"**Token**: %s\n"+
			"**Current Usage**: 100%%\n\n"+
			"The quota for this token has been fully exhausted. "+
			"All further requests will be rejected until the quota is reset or increased.",
		groupID,
		tokenName,
	)

	return title, message
}
