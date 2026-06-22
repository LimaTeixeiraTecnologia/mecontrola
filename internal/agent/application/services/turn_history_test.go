package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

type TurnHistorySuite struct {
	suite.Suite
	th TurnHistory
}

func TestTurnHistorySuite(t *testing.T) {
	suite.Run(t, new(TurnHistorySuite))
}

func (s *TurnHistorySuite) SetupTest() {
	s.th = TurnHistory{}
}

func (s *TurnHistorySuite) TestDeserializeEmpty() {
	type args struct {
		raw []byte
	}
	type dependencies struct{}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(turns []entities.ConversationMessage, err error)
	}{
		{
			name:         "nil input returns nil no error",
			args:         args{raw: nil},
			dependencies: dependencies{},
			expect: func(turns []entities.ConversationMessage, err error) {
				s.NoError(err)
				s.Nil(turns)
			},
		},
		{
			name:         "empty slice input returns nil no error",
			args:         args{raw: []byte{}},
			dependencies: dependencies{},
			expect: func(turns []entities.ConversationMessage, err error) {
				s.NoError(err)
				s.Nil(turns)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			turns, err := s.th.Deserialize(scenario.args.raw)
			scenario.expect(turns, err)
		})
	}
}

func (s *TurnHistorySuite) TestSerializeDeserializeRoundTrip() {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	type args struct {
		turns []entities.ConversationMessage
	}
	type dependencies struct{}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result []entities.ConversationMessage, err error)
	}{
		{
			name: "roundtrip preserves role and content",
			args: args{turns: []entities.ConversationMessage{
				{Role: "user", Content: "hello", At: now},
				{Role: "assistant", Content: "hi", At: now},
			}},
			dependencies: dependencies{},
			expect: func(result []entities.ConversationMessage, err error) {
				s.NoError(err)
				s.Len(result, 2)
				s.Equal("user", result[0].Role)
				s.Equal("hello", result[0].Content)
				s.Equal("assistant", result[1].Role)
				s.Equal("hi", result[1].Content)
			},
		},
		{
			name:         "empty turns serialize to bracket pair and deserialize to empty",
			args:         args{turns: nil},
			dependencies: dependencies{},
			expect: func(result []entities.ConversationMessage, err error) {
				s.NoError(err)
				s.Empty(result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			raw, serErr := s.th.Serialize(scenario.args.turns)
			s.NoError(serErr)
			result, err := s.th.Deserialize(raw)
			scenario.expect(result, err)
		})
	}
}

func (s *TurnHistorySuite) TestAppendWindowSliding() {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	type args struct {
		turns        []entities.ConversationMessage
		userMsg      string
		assistantMsg string
		maxPairs     int
	}
	type dependencies struct{}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result []entities.ConversationMessage)
	}{
		{
			name: "oldest pair dropped when exceeding maxPairs",
			args: args{
				turns: []entities.ConversationMessage{
					{Role: "user", Content: "first", At: now},
					{Role: "assistant", Content: "resp1", At: now},
					{Role: "user", Content: "second", At: now},
					{Role: "assistant", Content: "resp2", At: now},
				},
				userMsg:      "third",
				assistantMsg: "resp3",
				maxPairs:     2,
			},
			dependencies: dependencies{},
			expect: func(result []entities.ConversationMessage) {
				s.Len(result, 4)
				s.Equal("second", result[0].Content)
				s.Equal("resp2", result[1].Content)
				s.Equal("third", result[2].Content)
				s.Equal("resp3", result[3].Content)
			},
		},
		{
			name: "zero maxPairs disables windowing",
			args: args{
				turns: []entities.ConversationMessage{
					{Role: "user", Content: "a", At: now},
					{Role: "assistant", Content: "b", At: now},
				},
				userMsg:      "c",
				assistantMsg: "d",
				maxPairs:     0,
			},
			dependencies: dependencies{},
			expect: func(result []entities.ConversationMessage) {
				s.Len(result, 4)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			result := s.th.Append(scenario.args.turns, scenario.args.userMsg, scenario.args.assistantMsg, now, scenario.args.maxPairs)
			scenario.expect(result)
		})
	}
}

func (s *TurnHistorySuite) TestAppendExactWindow() {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	type args struct {
		turns        []entities.ConversationMessage
		userMsg      string
		assistantMsg string
		maxPairs     int
	}
	type dependencies struct{}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result []entities.ConversationMessage)
	}{
		{
			name: "exactly maxPairs keeps all messages",
			args: args{
				turns: []entities.ConversationMessage{
					{Role: "user", Content: "existing", At: now},
					{Role: "assistant", Content: "existing-resp", At: now},
				},
				userMsg:      "new",
				assistantMsg: "new-resp",
				maxPairs:     2,
			},
			dependencies: dependencies{},
			expect: func(result []entities.ConversationMessage) {
				s.Len(result, 4)
				s.Equal("existing", result[0].Content)
				s.Equal("existing-resp", result[1].Content)
				s.Equal("new", result[2].Content)
				s.Equal("new-resp", result[3].Content)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			result := s.th.Append(scenario.args.turns, scenario.args.userMsg, scenario.args.assistantMsg, now, scenario.args.maxPairs)
			scenario.expect(result)
		})
	}
}

func (s *TurnHistorySuite) TestToLLMMessages() {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	type args struct {
		turns []entities.ConversationMessage
	}
	type dependencies struct{}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
	}{
		{
			name: "maps role and content correctly ignoring At",
			args: args{turns: []entities.ConversationMessage{
				{Role: "user", Content: "hello world", At: now},
				{Role: "assistant", Content: "response here", At: now},
			}},
			dependencies: dependencies{},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			msgs := s.th.ToLLMMessages(scenario.args.turns)
			s.Len(msgs, len(scenario.args.turns))
			for i, m := range msgs {
				s.Equal(scenario.args.turns[i].Role, m.Role)
				s.Equal(scenario.args.turns[i].Content, m.Content)
			}
		})
	}
}
