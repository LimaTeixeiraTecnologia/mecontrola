package outbox

import (
	"fmt"
	"os"

	"github.com/google/uuid"
)

func newInstanceID() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString())
}
