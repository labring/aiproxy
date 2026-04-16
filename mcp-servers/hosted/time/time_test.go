//nolint:testpackage
package time

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalTimezoneConfigTemplateExampleIsValid(t *testing.T) {
	t.Parallel()

	template := configTemplates["local_timezone"]
	require.NotNil(t, template.Validator)
	require.NoError(t, template.Validator(template.Example))
}

func TestGetTimezoneSupportsIANAName(t *testing.T) {
	t.Parallel()

	loc, err := getTimezone("America/New_York")
	require.NoError(t, err)
	require.Equal(t, "America/New_York", loc.String())
}
