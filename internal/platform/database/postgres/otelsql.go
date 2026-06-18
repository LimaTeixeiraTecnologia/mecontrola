package postgres

import (
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
)

func instrumentDriver(driverName, dsn string) (*sql.DB, error) {
	db, err := otelsql.Open(driverName, dsn,
		otelsql.WithAttributes(
			attribute.String("db.system", "postgresql"),
		),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			Ping:                 true,
			RowsNext:             false,
			DisableQuery:         false,
			OmitRows:             true,
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open instrumented database: %w", err)
	}

	if _, err := otelsql.RegisterDBStatsMetrics(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to register database metrics: %w", err)
	}

	return db, nil
}
