package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthResolvePath_IsValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		path  AuthResolvePath
		valid bool
	}{
		{name: "identity", path: AuthResolvePathIdentity, valid: true},
		{name: "legacy", path: AuthResolvePathLegacy, valid: true},
		{name: "backfill", path: AuthResolvePathBackfill, valid: true},
		{name: "vazio", path: AuthResolvePath(""), valid: false},
		{name: "desconhecido", path: AuthResolvePath("miss"), valid: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.valid, tc.path.IsValid())
		})
	}
}

func TestAuthResolvePath_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "identity", AuthResolvePathIdentity.String())
	require.Equal(t, "legacy", AuthResolvePathLegacy.String())
	require.Equal(t, "backfill", AuthResolvePathBackfill.String())
}

func TestParseAuthResolvePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     string
		want    AuthResolvePath
		wantErr bool
	}{
		{name: "identity valido", raw: "identity", want: AuthResolvePathIdentity},
		{name: "legacy valido", raw: "legacy", want: AuthResolvePathLegacy},
		{name: "backfill valido", raw: "backfill", want: AuthResolvePathBackfill},
		{name: "vazio invalido", raw: "", wantErr: true},
		{name: "desconhecido invalido", raw: "miss", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAuthResolvePath(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
