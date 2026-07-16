package workflows

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type TreatmentNamePromptsSuite struct {
	suite.Suite
}

func TestTreatmentNamePromptsSuite(t *testing.T) {
	suite.Run(t, new(TreatmentNamePromptsSuite))
}

func (s *TreatmentNamePromptsSuite) TestTreatmentNameCapturePromptAsksTheOfficialQuestion() {
	s.Contains(treatmentNameCapturePrompt, "Antes da gente começar, como você gostaria que eu te chamasse? 💚")
	s.Contains(treatmentNameCapturePrompt, "Bem-vindo ao MeControla")
}

func (s *TreatmentNamePromptsSuite) TestTreatmentNameGoalPromptNoGreetingDoesNotRepeatWelcome() {
	s.NotContains(treatmentNameGoalPromptNoGreeting, "Bem-vindo ao MeControla")
	s.Contains(treatmentNameGoalPromptNoGreeting, "Qual é o seu principal objetivo financeiro")
}
