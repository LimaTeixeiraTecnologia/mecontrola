package services

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

var (
	spOnce sync.Once
	spLoc  *time.Location
)

func SaoPauloLocation() *time.Location {
	spOnce.Do(func() {
		loc, err := time.LoadLocation("America/Sao_Paulo")
		if err != nil {
			slog.Error("failed to load America/Sao_Paulo timezone", "error", err)
			os.Exit(1)
		}
		spLoc = loc
	})
	return spLoc
}

func MustLoadSaoPauloOrExit() {
	SaoPauloLocation()
}
