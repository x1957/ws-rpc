package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/x1957/ws-rpc/pkg/core"
)

type sub struct {
	Duration int
}

func add(ctx context.Context, arg sub) error {
	fmt.Printf("duration = %d\n", arg.Duration)
	conn := ctx.Value(core.ConnKey).(*core.WsConn)
	cnt := 0
	for range time.Tick(time.Duration(arg.Duration) * time.Second) {
		conn.Write([]byte(fmt.Sprintf("cnt = %d", cnt)))
		cnt++
	}
	return nil
}

func main() {
	flag.Parse()
	opts := core.NewDefaultWSOpts()
	s := core.NewWSServer(opts)
	s.Handle("add", add)
	s.Serve()
}
