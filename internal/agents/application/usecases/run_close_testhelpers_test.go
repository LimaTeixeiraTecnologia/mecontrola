package usecases

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
)

func fieldValue(fields []observability.Field, key string) (string, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f.StringValue(), true
		}
	}
	return "", false
}

func assertRunUpdateErrorLabels(t *testing.T, fields []observability.Field, workflow, stage, status string) {
	t.Helper()
	gotWorkflow, okWorkflow := fieldValue(fields, "workflow")
	require.True(t, okWorkflow, "label workflow ausente")
	require.Equal(t, workflow, gotWorkflow)

	gotStage, okStage := fieldValue(fields, "stage")
	require.True(t, okStage, "label stage ausente")
	require.Equal(t, stage, gotStage)

	gotStatus, okStatus := fieldValue(fields, "status")
	require.True(t, okStatus, "label status ausente")
	require.Equal(t, status, gotStatus)

	for _, f := range fields {
		switch f.Key {
		case "user_id", "wamid", "correlation_key", "resource_id", "thread_id", "category_id":
			require.Failf(t, "label sensivel", "metrica nao pode carregar label sensivel %q", f.Key)
		}
	}
}

func hasRunUpdateErrorLog(entries []fake.LogEntry) bool {
	for _, e := range entries {
		if _, ok := fieldValue(e.Fields, "run_id"); !ok {
			continue
		}
		if _, ok := fieldValue(e.Fields, "wamid"); !ok {
			continue
		}
		if _, ok := fieldValue(e.Fields, "workflow"); !ok {
			continue
		}
		if _, ok := fieldValue(e.Fields, "stage"); ok {
			return true
		}
	}
	return false
}
