//go:build integration

package testharness

import (
	"sync/atomic"
	"time"
)

// Clock is the time the hub's automations run by: the system clock moved by
// an offset a test sets. It runs at the real rate, so a test can put the hub
// a few seconds short of a time it wants to see the hub reach, such as a
// schedule's due minute, and wait those seconds rather than for the wall
// clock to get there.
type Clock struct {
	offset atomic.Int64
}

// Now is the hub's time.
func (c *Clock) Now() time.Time {
	return time.Now().Add(time.Duration(c.offset.Load()))
}

// Set moves the hub's time so that it reads at now, and running on from there.
func (c *Clock) Set(at time.Time) {
	c.offset.Store(int64(time.Until(at)))
}

// Reset puts the hub back on the system clock.
func (c *Clock) Reset() {
	c.offset.Store(0)
}
