package core

import (
	"context"
	"reflect"
)

const (
	Normal = 1
	Stream = 2
)

type handlerType struct {
	method     reflect.Value
	methodType int
}

func (s *WSServer) Handle(name string, handler interface{}) {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		panic("error: " + name + " method type not func.")
	}

	if t.NumIn() != 2 || !(t.NumOut() == 1 || t.NumOut() == 2) {
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

	var output reflect.Type
	var methodType int
	if t.NumOut() == 1 {
		output = t.Out(0)
		methodType = Stream
	} else {
		output = t.Out(1)
		methodType = Normal
	}
	// check output, error
	if !isImpl(output, reflect.TypeOf((*error)(nil)).Elem()) {
		panic("The second output must a type of error.")
	}

	_, loaded := s.handlers.LoadOrStore(name, handlerType{reflect.ValueOf(handler), methodType})
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
