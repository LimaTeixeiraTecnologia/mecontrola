package postgres

import "time"

type Option func(*Database)

func WithMaxOpenConns(n int) Option {
	return func(d *Database) {
		d.db.SetMaxOpenConns(n)
	}
}

func WithMaxIdleConns(n int) Option {
	return func(d *Database) {
		d.db.SetMaxIdleConns(n)
	}
}

func WithConnMaxLifetime(d time.Duration) Option {
	return func(db *Database) {
		db.db.SetConnMaxLifetime(d)
	}
}

func WithConnMaxIdleTime(d time.Duration) Option {
	return func(db *Database) {
		db.db.SetConnMaxIdleTime(d)
	}
}

func WithPoolConfig(maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) Option {
	return func(db *Database) {
		db.db.SetMaxOpenConns(maxOpen)
		db.db.SetMaxIdleConns(maxIdle)
		db.db.SetConnMaxLifetime(maxLifetime)
		db.db.SetConnMaxIdleTime(maxIdleTime)
	}
}
