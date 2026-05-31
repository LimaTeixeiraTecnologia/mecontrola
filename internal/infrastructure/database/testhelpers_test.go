package database_test

import "github.com/LimaTeixeiraTecnologia/mecontrola/configs"

// badDBConfig returns a configs.Config pointing to a non-existent Postgres
// server on localhost:1, guaranteed to refuse connections.
func badDBConfig() *configs.Config {
	return &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     "127.0.0.1",
			Port:     1,
			User:     "nobody",
			Password: "nopassword",
			Name:     "nodb",
			SSLMode:  "disable",
		},
	}
}
