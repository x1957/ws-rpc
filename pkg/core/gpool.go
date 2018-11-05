package core

import (
	"time"

	"github.com/golang/glog"
)

type gpool struct {
	size        int
	limit       int
	stop        chan struct{}
	workerQueue chan worker
}

type worker struct {
	f           func()
	timeInQueue time.Time
}

type poolBlockError struct{}

func (e *poolBlockError) Error() string {
	return "gpool blocked more than 2 seconds"
}

func newGpool(limit int) *gpool {
	g := &gpool{}
	g.limit = limit
	g.size = limit
	g.stop = make(chan struct{})
	g.workerQueue = make(chan worker, g.size)
	for i := 0; i < g.size; i++ {
		go g.runFunc()
	}
	return g
}

func (gpool *gpool) run(f func()) error {
	worker := worker{
		f:           f,
		timeInQueue: time.Now(),
	}
	select {
	case gpool.workerQueue <- worker:
		//do nothing
	case time.After(2 * time.Second):
		return &poolBlockError{}
	}
	return nil
}

func (gpool *gpool) runFunc() {
	for {
		select {
		case <-gpool.stop:
			// TODO do more
			return
		case worker := <-gpool.workerQueue:
			// TODO calc the time in queue
			runWorker(worker)
		}
	}
}

func runWorker(worker worker) {
	defer func() {
		if x := recover(); x != nil {
			glog.Errorf("run func error. %v", x)
		}
	}()
	worker.f()
}
