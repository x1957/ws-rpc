package core

import "sync"

type gpool struct {
	sync.Mutex
	size        int
	limit       int
	stop        chan struct{}
	workerQueue chan func()
}

func newGpool(limit int) *gpool {
	g := &gpool{}
	g.limit = limit
	g.size = limit
	g.stop = make(chan struct{})
	g.workerQueue = make(chan func(), g.size)
	for i := 0; i < g.size; i++ {
		go g.runFunc()
	}
	return g
}

func (gpool *gpool) run(f func()) error {
	gpool.workerQueue <- f
	return nil
}

func (gpool *gpool) runFunc() {
	for {
		select {
		case <-gpool.stop:
			// TODO do more
			return
		case f := <-gpool.workerQueue:
			f()
		}
	}
}
