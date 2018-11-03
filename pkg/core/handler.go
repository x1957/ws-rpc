package core

import (
	"context"
	"reflect"
)

func (s *WSServer) Handle(name string, handler interface{}) {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		panic("error: " + name + " method type not func.")
	}

	if t.NumIn() != 2 || t.NumOut() != 2 {
		panic("error: handler wants 2 input and 2 output parameters.")
	}

	// check input, context and struct
	arg1 := t.In(0)
	// reflect.TypeOf((*context.Context)(nil)).Elem()
	if !isImpl(arg1, reflect.TypeOf((*context.Context)(nil)).Elem()) {
		panic("The first arg must context.Context")
	}
	arg2 := t.In(1)
	if arg2.Kind() != reflect.Struct {
		panic("The second arg must a type of struct.")
	}

	// check output, error
	output2 := t.Out(1)
	if !isImpl(output2, reflect.TypeOf((*error)(nil)).Elem()) {
		panic("The second output must a type of error.")
	}

	_, loaded := s.handlers.LoadOrStore(name, reflect.ValueOf(handler))
	if loaded {
		panic("method " + name + " already exists.")
	}
}

func isImpl(t reflect.Type, rt reflect.Type) (b bool) {
	defer func() {
		if r := recover(); r != nil {
			b = false
		}
	}()
	b = t.Implements(rt)
	return
}

type HandlerNotExistError struct {
	method string
}

func (e *HandlerNotExistError) Error() string {
	return e.method + " handler not exists."
}

func (s *WSServer) getHandler(method string) (reflect.Value, error) {
	handler, ok := s.handlers.Load(method)
	if !ok {
		return reflect.Value{}, &HandlerNotExistError{method}
	}
	return handler.(reflect.Value), nil
}
