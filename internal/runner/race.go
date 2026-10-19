package runner

import (
	"math/rand/v2"
	"sync"
	"time"
)

type Barrier struct {
	arrived sync.WaitGroup
	release chan struct{}
	once    sync.Once
	proceed bool
}

func NewBarrier(n int) *Barrier {
	b := &Barrier{release: make(chan struct{})}
	b.arrived.Add(n)
	return b
}

func (b *Barrier) Wait() bool {
	b.arrived.Done()
	<-b.release
	return b.proceed
}

func (b *Barrier) WaitAll() {
	b.arrived.Wait()
}

func (b *Barrier) Release(proceed bool) {
	b.once.Do(func() {
		b.proceed = proceed
		close(b.release)
	})
}

func Jitter(seed int64, run, n int, lo, hi time.Duration) []time.Duration {
	out := make([]time.Duration, n)
	if hi <= lo {
		for i := range out {
			out[i] = lo
		}
		return out
	}
	r := rand.New(rand.NewPCG(uint64(seed), uint64(run)))
	span := int64(hi - lo)
	for i := range out {
		out[i] = lo + time.Duration(r.Int64N(span+1))
	}
	return out
}
