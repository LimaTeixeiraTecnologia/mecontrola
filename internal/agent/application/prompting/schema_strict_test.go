package prompting_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
)

func TestParseIntentJSONSchema_StrictComplete(t *testing.T) {
	t.Parallel()
	assertStrictObjectSchema(t, "ParseIntentJSONSchema", prompting.ParseIntentJSONSchema())
}

func assertStrictObjectSchema(t *testing.T, path string, schema map[string]any) {
	t.Helper()

	if !isObjectType(schema["type"]) {
		assertStrictNested(t, path, schema)
		return
	}

	properties, ok := schema["properties"].(map[string]any)
	require.Truef(t, ok, "%s: object schema must declare properties", path)

	additional, ok := schema["additionalProperties"].(bool)
	require.Truef(t, ok, "%s: object schema must declare additionalProperties as bool", path)
	require.Falsef(t, additional, "%s: additionalProperties must be false", path)

	requiredRaw, ok := schema["required"].([]string)
	require.Truef(t, ok, "%s: object schema must declare required as []string", path)

	requiredSet := make(map[string]struct{}, len(requiredRaw))
	for _, key := range requiredRaw {
		requiredSet[key] = struct{}{}
	}

	for key := range properties {
		_, present := requiredSet[key]
		require.Truef(t, present, "%s: property %q is missing from required", path, key)
	}
	for key := range requiredSet {
		_, present := properties[key]
		require.Truef(t, present, "%s: required key %q is missing from properties", path, key)
	}
	require.Lenf(t, requiredRaw, len(properties), "%s: required count must equal properties count", path)

	for key, value := range properties {
		if nested, ok := value.(map[string]any); ok {
			assertStrictNested(t, path+"."+key, nested)
		}
	}
}

func assertStrictNested(t *testing.T, path string, schema map[string]any) {
	t.Helper()

	if isObjectType(schema["type"]) {
		assertStrictObjectSchema(t, path, schema)
		return
	}
	if items, ok := schema["items"].(map[string]any); ok {
		assertStrictNested(t, path+".items", items)
	}
}

func isObjectType(raw any) bool {
	switch typed := raw.(type) {
	case string:
		return typed == "object"
	case []string:
		for _, candidate := range typed {
			if candidate == "object" {
				return true
			}
		}
	}
	return false
}
