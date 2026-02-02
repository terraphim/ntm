package events

import (
	"log/slog"
	"sync"
	"sync/atomic"
)

// EventEmitter provides non-blocking emission of BusEvents to an EventBus.
//
// Design notes:
// - Emit() never blocks callers (drops when buffer is full).
// - Events are published on a single worker goroutine.
// - Consumers should subscribe via EventBus subscriptions.
type EventEmitter struct {
	bus *EventBus
	ch  chan BusEvent

	dropped atomic.Int64

	startOnce sync.Once
}

// NewEventEmitter creates an emitter for the given bus. If bus is nil, DefaultBus is used.
func NewEventEmitter(bus *EventBus, buffer int) *EventEmitter {
	if bus == nil {
		bus = DefaultBus
	}
	if buffer < 1 {
		buffer = 256
	}
	return &EventEmitter{
		bus: bus,
		ch:  make(chan BusEvent, buffer),
	}
}

// Start launches the background publisher loop (idempotent).
func (e *EventEmitter) Start() {
	e.startOnce.Do(func() {
		go func() {
			for ev := range e.ch {
				e.bus.Publish(ev)
			}
		}()
	})
}

// Emit enqueues an event for async publish. If the buffer is full, the event is dropped.
func (e *EventEmitter) Emit(ev BusEvent) {
	if ev == nil {
		return
	}
	e.Start()
	select {
	case e.ch <- ev:
	default:
		n := e.dropped.Add(1)
		// Avoid log spam: emit only for the first drop and then every 1000 drops.
		if n == 1 || n%1000 == 0 {
			slog.Default().Debug("event emitter dropped events (buffer full)", "dropped", n, "event_type", ev.EventType())
		}
	}
}

// Dropped returns the number of dropped events.
func (e *EventEmitter) Dropped() int64 {
	return e.dropped.Load()
}

var (
	defaultEmitterOnce sync.Once
	defaultEmitter     *EventEmitter
)

// DefaultEmitter returns the global default emitter publishing into DefaultBus.
func DefaultEmitter() *EventEmitter {
	defaultEmitterOnce.Do(func() {
		defaultEmitter = NewEventEmitter(DefaultBus, 1024)
	})
	return defaultEmitter
}
