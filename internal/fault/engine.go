package fault

import (
	"math/rand"
	"sync"
	"time"
)

type Engine struct {
	crashed    bool
	dropRate   float64
	delayMs    int
	partitions map[string]bool
	mu         sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		partitions: make(map[string]bool),
	}
}

// Crash control
func (e *Engine) Crash() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.crashed = true
}

func (e *Engine) Recover() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.crashed = false
}

func (e *Engine) IsCrashed() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.crashed
}


// Drop model
func (e *Engine) SetDropRate(p float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dropRate = p
}

func (e *Engine) ShouldDrop() bool {
	e.mu.RLock()
	p := e.dropRate
	e.mu.RUnlock()
	return rand.Float64() < p
}


// Delay model
func (e *Engine) SetDelay(ms int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.delayMs = ms
}

func (e *Engine) ApplyDelay() {
	e.mu.RLock()
	d := e.delayMs
	e.mu.RUnlock()
	if d > 0 {
		time.Sleep(time.Duration(d) * time.Millisecond)
	}
}

// Partition model
func (e *Engine) Partition(peer string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.partitions[peer] = true
}

func (e *Engine) Heal(peer string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.partitions, peer)
}

func (e *Engine) IsPartitioned(peer string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.partitions[peer]
}

/*
Partition is directional.
Meaning:
node A partitioned from B
but B may still see A.
*/