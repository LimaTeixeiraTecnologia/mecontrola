//go:build e2e

package e2e_test

import (
	"github.com/cucumber/godog"
)

func registerSteps(sc *godog.ScenarioContext, e *e2eCtx) {
	registerIdentitySteps(sc, e)
	registerIdentityExtSteps(sc, e)
}
