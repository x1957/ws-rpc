package core

import "sync"

var handlerCache sync.Map

func HandleFunc(method string, f MethodHandlerFunc) {
	Handle(method, MethodHandler(f))
}

func Handle(method string, handler MethodHandler) {
	_, loaded := handlerCache.LoadOrStore(method, handler)
	if loaded {
		panic("method " + method + " already exists.")
	}
}

type HandlerNotExistError struct {
	method string
}

func (e *HandlerNotExistError) Error() string {
	return e.method + " handler not exists."
}

func getHandler(method string) (MethodHandler, error) {
	handler, ok := handlerCache.Load(method)
	if !ok {
		return nil, &HandlerNotExistError{method}
	}
	return handler.(MethodHandler), nil
}
