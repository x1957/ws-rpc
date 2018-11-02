package core

import "context"

type MethodHandler interface {
	Call(ctx context.Context, args []byte) ([]byte, error)
}

type MethodHandlerFunc func(ctx context.Context, args []byte) ([]byte, error)

func (f MethodHandlerFunc) Call(ctx context.Context, args []byte) ([]byte, error) {
	return f(ctx, args)
}
