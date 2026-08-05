package input_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
)

func TestAudioDecisionInputValidate(t *testing.T) {
	confidenceOK := 0.9
	confidenceOutOfRange := 1.5

	scenarios := []struct {
		name    string
		in      input.AudioDecisionInput
		wantErr bool
	}{
		{
			name:    "zero value is invalid",
			in:      input.AudioDecisionInput{},
			wantErr: true,
		},
		{
			name: "missing model is invalid",
			in: input.AudioDecisionInput{
				Text:          "gastei 50 reais no mercado",
				Language:      "pt",
				MinConfidence: 0.8,
			},
			wantErr: true,
		},
		{
			name: "min_confidence out of range is invalid",
			in: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				MinConfidence: 1.2,
			},
			wantErr: true,
		},
		{
			name: "negative min_confidence is invalid",
			in: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				MinConfidence: -0.1,
			},
			wantErr: true,
		},
		{
			name: "confidence out of range is invalid",
			in: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				MinConfidence: 0.8,
				Confidence:    &confidenceOutOfRange,
			},
			wantErr: true,
		},
		{
			name: "valid input with confidence",
			in: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "gastei 50 reais no mercado",
				Language:      "pt",
				MinConfidence: 0.8,
				Confidence:    &confidenceOK,
			},
			wantErr: false,
		},
		{
			name: "valid input with stt error and empty text",
			in: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				MinConfidence: 0.8,
			},
			wantErr: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.in.Validate()
			if scenario.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
