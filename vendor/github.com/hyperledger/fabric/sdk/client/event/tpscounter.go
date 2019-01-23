/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package event

import (
	"sync"
	"time"
)

type tpsCounter struct {
	sync.Mutex
	counter uint64
	output  chan uint64
}

func (tc *tpsCounter) accumulate(count uint64) {
	tc.Lock()
	defer tc.Unlock()
	tc.counter = tc.counter + count
}

func (tc *tpsCounter) getAccumulation() uint64 {
	tc.Lock()
	defer tc.Unlock()
	al := tc.counter
	tc.counter = 0
	return al
}

func (tc *tpsCounter) getOutput() <-chan uint64 {
	return tc.output
}

func calculateTps() *tpsCounter {
	tc := &tpsCounter{
		counter: 0,
		output:  make(chan uint64),
	}

	go func() {
		for {
			time.Sleep(time.Second)
			go func() { tc.output <- tc.getAccumulation() }()
		}
	}()
	return tc
}
