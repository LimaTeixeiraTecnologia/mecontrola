package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
)

type mockMessageIndexPublisher struct {
	mock.Mock
}

func (m *mockMessageIndexPublisher) PublishIndex(ctx context.Context, p memory.IndexMessagePayload) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

type PublishingMessageStoreSuite struct {
	suite.Suite
	ctx      context.Context
	storeMck *mocks.MessageStore
	pubMck   *mockMessageIndexPublisher
}

func TestPublishingMessageStoreSuite(t *testing.T) {
	suite.Run(t, new(PublishingMessageStoreSuite))
}

func (s *PublishingMessageStoreSuite) SetupTest() {
	s.ctx = context.Background()
	s.storeMck = mocks.NewMessageStore(s.T())
	s.pubMck = &mockMessageIndexPublisher{}
}

func (s *PublishingMessageStoreSuite) TestAppend() {
	const model = "text-embedding-3-small"

	threadPK := uuid.New()
	msg := memory.Message{
		ID:               uuid.New(),
		PlatformThreadID: threadPK,
		ResourceID:       "res-1",
		Role:             memory.RoleUser,
		Content:          "hello world",
	}

	type args struct {
		threadPK uuid.UUID
		msg      memory.Message
	}
	type deps struct {
		store *mocks.MessageStore
		pub   *mockMessageIndexPublisher
	}

	scenarios := []struct {
		name   string
		args   args
		deps   deps
		expect func(err error)
	}{
		{
			name: "deve publicar evento apos append bem-sucedido",
			args: args{threadPK: threadPK, msg: msg},
			deps: deps{
				store: func() *mocks.MessageStore {
					s.storeMck.EXPECT().Append(s.ctx, threadPK, msg).Return(nil).Once()
					return s.storeMck
				}(),
				pub: func() *mockMessageIndexPublisher {
					s.pubMck.On("PublishIndex", s.ctx, mock.MatchedBy(func(p memory.IndexMessagePayload) bool {
						return p.ResourceID == msg.ResourceID &&
							p.ThreadID == threadPK.String() &&
							p.MessageID == msg.ID &&
							p.Content == msg.Content &&
							p.Model == model
					})).Return(nil).Once()
					return s.pubMck
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando append falha sem publicar",
			args: args{threadPK: threadPK, msg: msg},
			deps: deps{
				store: func() *mocks.MessageStore {
					s.storeMck.EXPECT().Append(s.ctx, threadPK, msg).Return(errors.New("db error")).Once()
					return s.storeMck
				}(),
				pub: s.pubMck,
			},
			expect: func(err error) {
				s.Error(err)
				s.pubMck.AssertNotCalled(s.T(), "PublishIndex")
			},
		},
		{
			name: "deve retornar nil quando publicacao falha (degradacao controlada)",
			args: args{threadPK: threadPK, msg: msg},
			deps: deps{
				store: func() *mocks.MessageStore {
					s.storeMck.EXPECT().Append(s.ctx, threadPK, msg).Return(nil).Once()
					return s.storeMck
				}(),
				pub: func() *mockMessageIndexPublisher {
					s.pubMck.On("PublishIndex", s.ctx, mock.Anything).Return(errors.New("outbox error")).Once()
					return s.pubMck
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := memory.NewPublishingMessageStore(scenario.deps.store, scenario.deps.pub, model, fake.NewProvider())
			err := sut.Append(s.ctx, scenario.args.threadPK, scenario.args.msg)
			scenario.expect(err)
		})
	}
}

func (s *PublishingMessageStoreSuite) TestRecent_Delegates() {
	threadPK := uuid.New()
	expected := []memory.Message{{ID: uuid.New(), Content: "hello"}}

	s.storeMck.EXPECT().Recent(s.ctx, threadPK, 5).Return(expected, nil).Once()

	sut := memory.NewPublishingMessageStore(s.storeMck, s.pubMck, "model", fake.NewProvider())
	msgs, err := sut.Recent(s.ctx, threadPK, 5)

	s.NoError(err)
	s.Equal(expected, msgs)
}
