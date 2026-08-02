package publish

import (
	"context"
	"errors"
)

var ErrAlreadyExists = errors.New("object already exists")

type PutOptions struct {
	Immutable bool
}

type ObjectStore interface {
	Put(context.Context, string, []byte, PutOptions) error
}
