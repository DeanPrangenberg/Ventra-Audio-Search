package globalUtils

// NoneStackingEvent is a synchronization primitive that allows you to trigger an event without stacking multiple events if they occur in quick succession.
type NoneStackingEvent struct {
	ch chan struct{}
}

// NewSignal creates a new NoneStackingEvent.
func NewSignal() *NoneStackingEvent {
	return &NoneStackingEvent{ch: make(chan struct{}, 1)}
}

// Trigger sends a signal if there isn't already one pending. This ensures that if multiple events happen in quick succession, only one signal will be sent until the receiver has processed it.
func (s *NoneStackingEvent) Trigger() {
	select {
	case s.ch <- struct{}{}:
	default:
	}
}

// Reader returns a read-only channel that can be used to receive signals. The receiver should read from this channel to process the signal and allow new signals to be sent.
func (s *NoneStackingEvent) Reader() <-chan struct{} {
	return s.ch
}
