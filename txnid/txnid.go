package txnid

import (
	"fmt"
	"sync"
)

// Numbers holds the counter variables. We'll turn this into a singleton in the InitNumbers function.
// lock: mutex to keep this all nice and threadsafe
// base: the upper 48 bytes of the number generator
// counter: the lower 16 bytes that are constantly incremented
// increment: what we keep adding to counter
// stop: when true, stop serving numbers. This is permanent and triggered by Snapshot()
type Numbers struct {
	lock      sync.Mutex
	base      uint64
	counter   uint64
	increment uint16
	_stop     bool
}

// tickOver: when Numbers.counter reaches "tick over", we increment Numbers.base by 1,
// and subtract tickOver+1 from counter.
const (
	tickOver uint64 = 1 << 16
)

// numgen = holds the singleton Numbers struct
// once = used to implement our singleton
var (
	numgen Numbers
	once   sync.Once
)

// When we tick over (see tickOver), we call back to this optional function
// Used by the txn-id-server to store progress
// NOTE:  This will be holding a mutex lock on the number generator, so you cannot
// generate other numbers within this function.
// BE CAREFUL AND PLAY NICE. ALL WARRANTIES DEFINITELY VOIDED HERE!
var (
	RollOverCallBack func(b uint64) = nil
)

// InitNumbers initialises the singleton Numbers struct (or returns the previously initialised version)
func InitNumbers(b, c uint64, i uint16) *Numbers {

	once.Do(func() {
		numgen = Numbers{base: b << 16, counter: c, increment: i}
	})
	return &numgen
}

// GetNextNum - returns the next number in the sequence. Mutex locked to prevent multi-threaded apps from clobbering each other.
func (n *Numbers) GetNextNum() (z uint64, stopped bool) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n._stop {
		z = 0
	} else {
		z = n.base | n.counter

		n.counter += uint64(n.increment)

		if n.counter >= tickOver {
			n.counter -= tickOver
			n.base += tickOver

			if RollOverCallBack != nil {
				RollOverCallBack(n.base >> 16)
			}
		}
	}

	return z, n._stop
}

// stop() Non-thread-safe procedure to set the stop flag.
// Used internally by the Snapshot() and Stop() functions
func (n *Numbers) stop() {
	n._stop = true
	return
}

// Stop() Thread-safe procedure to set the stop flag.
func (n *Numbers) Stop() {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.stop()
	return
}

// restart() Non-thread-safe procedure to clear the stop flag.
func (n *Numbers) restart() {
	n._stop = false
	return
}

// Restart() Thread-safe procedure to clear the stop flag.
func (n *Numbers) Restart() {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.restart()
	return
}

// GetCurrent - get the current state of the number generator.
// stop - if set to true, stops the number generate too
// Can be used to implement a "save" function.
func (n *Numbers) Snapshot(stop bool) (base uint64, counter uint64, increment uint16) {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Time to stop it seems
	if stop {
		n.stop()
	}

	return n.base >> 16, n.counter, n.increment
}

// Stringify!
func (n *Numbers) String() string {
	b, c, i := n.Snapshot(false)

	return fmt.Sprintf("Number generator: Base: %d, Counter: %d, Increment: %d -> Current: %d", b, c, i, (b<<16)|c)
}
