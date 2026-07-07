package agent

import "context"

type identityKey struct{}

type invocationItemSeqKey struct{}

type toolIdentity struct {
	userID   string
	threadID string
	wamid    string
	itemSeq  int
}

func withToolIdentity(ctx context.Context, in InboundRequest) context.Context {
	return context.WithValue(ctx, identityKey{}, &toolIdentity{
		userID:   in.ResourceID,
		threadID: in.ThreadID,
		wamid:    in.MessageID,
	})
}

func InboundExecutionFromContext(ctx context.Context) (resourceID, threadID, messageID string, itemSeq int, ok bool) {
	identity, hasIdentity := ctx.Value(identityKey{}).(*toolIdentity)
	if !hasIdentity {
		return "", "", "", 0, false
	}
	seq, _ := ctx.Value(invocationItemSeqKey{}).(int)
	return identity.userID, identity.threadID, identity.wamid, seq, true
}

func InboundIdentityFromContext(ctx context.Context) (resourceID, messageID string, itemSeq int, ok bool) {
	resourceID, _, messageID, itemSeq, ok = InboundExecutionFromContext(ctx)
	return
}

func WithToolInvocationContext(ctx context.Context, resourceID, messageID string, itemSeq int) context.Context {
	ctx = context.WithValue(ctx, identityKey{}, &toolIdentity{
		userID:   resourceID,
		threadID: "",
		wamid:    messageID,
		itemSeq:  itemSeq,
	})
	return context.WithValue(ctx, invocationItemSeqKey{}, itemSeq)
}
