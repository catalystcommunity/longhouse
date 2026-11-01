package store

import "time"

// EventFilter selects events that overlap the given range. Either bound
// can be nil. RangeStart is inclusive; RangeEnd is exclusive.
type EventFilter struct {
	HouseID    string
	RangeStart *time.Time
	RangeEnd   *time.Time
}
