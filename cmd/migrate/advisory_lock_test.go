package migrate

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAcquireMigrationLock(t *testing.T) {
	scenarios := []struct {
		name        string
		setupMock   func(sqlmock.Sqlmock)
		expectError bool
		errorMsg    string
	}{
		{
			name: "deve adquirir lock com sucesso",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT pg_try_advisory_lock\\(\\$1\\)").
					WithArgs(int64(424242)).
					WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
			},
			expectError: false,
		},
		{
			name: "deve retornar erro quando lock ja esta em uso",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT pg_try_advisory_lock\\(\\$1\\)").
					WithArgs(int64(424242)).
					WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(false))
			},
			expectError: true,
			errorMsg:    "outro processo de migrate esta em execucao",
		},
		{
			name: "deve retornar erro quando query falha",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT pg_try_advisory_lock\\(\\$1\\)").
					WithArgs(int64(424242)).
					WillReturnError(errors.New("connection refused"))
			},
			expectError: true,
			errorMsg:    "advisory lock query: connection refused",
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			scenario.setupMock(mock)

			unlock, err := acquireMigrationLock(context.Background(), db)
			if scenario.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), scenario.errorMsg)
				require.Nil(t, unlock)
				require.NoError(t, mock.ExpectationsWereMet())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, unlock)

			mock.ExpectExec("SELECT pg_advisory_unlock\\(\\$1\\)").
				WithArgs(int64(424242)).
				WillReturnResult(sqlmock.NewResult(0, 0))

			unlock()
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
