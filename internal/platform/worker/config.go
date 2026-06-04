package worker

import "time"

type Config struct {
	ShutdownTimeout time.Duration
}

func defaultConfig() Config {
	return Config{
		ShutdownTimeout: 30 * time.Second,
	}
}
