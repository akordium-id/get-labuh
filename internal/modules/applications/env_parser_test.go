package applications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEnvContent(t *testing.T) {
	raw := `
# This is a comment
APP_NAME="Get Labuh"
APP_ENV=production
export PORT=8080

# Secrets should be auto-detected
DATABASE_PASSWORD='secret_password_123'
API_SECRET_KEY="super-secret-token"

# Empty and spaces
EMPTY_VAL=
SPACED_VAR = spaced value
`

	vars := ParseEnvContent(raw, false)
	assert.Len(t, vars, 7)

	expected := map[string]struct {
		val      string
		isSecret bool
	}{
		"APP_NAME":          {"Get Labuh", false},
		"APP_ENV":           {"production", false},
		"PORT":              {"8080", false},
		"DATABASE_PASSWORD": {"secret_password_123", true},
		"API_SECRET_KEY":    {"super-secret-token", true},
		"EMPTY_VAL":         {"", false},
		"SPACED_VAR":        {"spaced value", false},
	}

	for _, v := range vars {
		exp, exists := expected[v.Key]
		assert.True(t, exists, "key should exist: %s", v.Key)
		assert.Equal(t, exp.val, v.Value, "value for %s", v.Key)
		assert.Equal(t, exp.isSecret, v.IsSecret, "isSecret for %s", v.Key)
	}

	// Test forceSecret flag
	forcedVars := ParseEnvContent("SIMPLE_VAR=hello", true)
	assert.Len(t, forcedVars, 1)
	assert.Equal(t, "SIMPLE_VAR", forcedVars[0].Key)
	assert.Equal(t, "hello", forcedVars[0].Value)
	assert.True(t, forcedVars[0].IsSecret)
}
