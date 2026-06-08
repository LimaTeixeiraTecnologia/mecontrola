package interfaces

import "context"

type TokenCipher interface {
	Encrypt(ctx context.Context, clearToken string) (string, error)
	Decrypt(ctx context.Context, ciphertext string) (string, error)
}
