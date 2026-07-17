package workflows

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type stubMessageStore struct {
	appended []memory.Message
	recent   []memory.Message
}

func (s *stubMessageStore) Append(_ context.Context, _ uuid.UUID, m memory.Message) error {
	s.appended = append(s.appended, m)
	return nil
}

func (s *stubMessageStore) Recent(_ context.Context, _ uuid.UUID, _ int) ([]memory.Message, error) {
	return s.recent, nil
}

func TestHistoryHygieneStoreFiltersOrientation(t *testing.T) {
	next := &stubMessageStore{recent: []memory.Message{
		{Role: memory.RoleUser, Content: "Gastei 500 no mercado"},
		{Role: memory.RoleAssistant, Content: MultiItemOrientationMessage},
		{Role: memory.RoleUser, Content: "Gastei 20 no cinema"},
		{Role: memory.RoleAssistant, Content: "Prontinho! ✅"},
	}}

	store := NewHistoryHygieneStore(next)
	got, err := store.Recent(context.Background(), uuid.New(), 20)
	if err != nil {
		t.Fatalf("Recent erro: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("esperava 3 mensagens após hygiene, obteve %d", len(got))
	}
	for _, m := range got {
		if m.Content == MultiItemOrientationMessage {
			t.Fatal("orientação de múltiplos lançamentos não deveria voltar no histórico")
		}
	}
}

func TestHistoryHygieneStoreAppendDelegates(t *testing.T) {
	next := &stubMessageStore{}
	store := NewHistoryHygieneStore(next)

	if err := store.Append(context.Background(), uuid.New(), memory.Message{Content: "oi"}); err != nil {
		t.Fatalf("Append erro: %v", err)
	}
	if len(next.appended) != 1 {
		t.Fatalf("esperava delegação do Append, obteve %d", len(next.appended))
	}
}
