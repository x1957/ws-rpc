package main

import (
	"context"
	"flag"

	"github.com/x1957/ws-rpc/pkg/core"
)

type helloArg struct {
	Name string
}

func hello(ctx context.Context, arg helloArg) (string, error) {
	return "hello " + arg.Name, nil
}

func main() {
	flag.Parse()
	opts := core.NewDefaultWSOpts()
	s := core.NewWSServer(opts)
	s.Handle("hello", hello)
	s.Serve()
}
