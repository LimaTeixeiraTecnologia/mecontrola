package id

import "github.com/google/uuid"

type Generator interface {
	NewID() string
}

type UUIDGenerator struct{}

func NewUUIDGenerator() UUIDGenerator {
	return UUIDGenerator{}
}

func (UUIDGenerator) NewID() string {
	return uuid.NewString()
}
