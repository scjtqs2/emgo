package noos

import (
	"sync/atomic"
	"sync/barrier"
	"unsafe"
	"syscall"
)

// An Event represents an event that gorutine or ISR can send and gorutine (but
// not ISR) can wait for. Events are intended for use by low-level library
// rutines to implement higher level communication and synchronization primitives
// like channels and mutexes.
type Event uintptr

const eventBits = uint32(unsafe.Sizeof(Event(0)) * 8)

var (
	eventReg Event
	gen      uint32
)

// AssignEvent returns event from some internal event pool.
// There is no any guarantee that subsequent calls of AssignEvent assigns
// different events, which means that AssignEvent can return Event already
// assigned by current or another gorutine.
func AssignEvent() Event {
	u := atomic.AddUint32(&gen, 1)
	return Event(1) << (u & (eventBits - 1))
}

// AssignEventFlag works like AssignEvent but guarantees that the least
// significant bit of returned value is zero.
func AssignEventFlag() Event {
	u := atomic.AddUint32(&gen, 1)
	return Event(2) << (u % (eventBits - 1))
}

// Send sends event that means it waking up all gorutines that wait for e.
// If some gorutine isn't waiting for any event, e is saved for this gorutine
// for possible future call of Wait. Compiler doesn't reorder Send with any
// memory operation that is before it in the program code.
func (e Event) Send() {
	barrier.Compiler()
	atomic.OrUintptr((*uintptr)(&eventReg), uintptr(e))
}

// Wait waits for event.
// If e == 0 it returns immediately. Wait clears all saved events for current
// gorutine so the information about sended events, that Wait hasn't waited for,
// is lost. Compiler doesn't reorder Wait with any memory operation that is
// before or after it in the program code.
func (e Event) Wait() {
	syscall.EventWait(uintptr(e))
}

// Sum returns logical sum of events.
// Sending the sum of events is equal to send all that events at once. Waiting
// for sum of events means waiting for at least one event from sum.
func (e Event) Sum(a Event) Event {
	return e | a
}
