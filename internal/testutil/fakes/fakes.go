// Package fakes holds hand-written in-memory implementations of the repository, index, storage,
// cache and queue interfaces for unit tests. Nothing here talks to the network.
//
// Every fake has an Errs field: set Errs["Method"] to make that method fail.
// "Not found" lookups return pgx.ErrNoRows, like the Postgres repositories do.
package fakes

import (
	"sync"

	"github.com/ak-ansari/mytube/pkg/logger"
)

// Errs injects errors per method name.
type Errs struct {
	mu sync.Mutex
	m  map[string]error
}

// Set makes method fail with err (nil clears it).
func (e *Errs) Set(method string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.m == nil {
		e.m = map[string]error{}
	}
	e.m[method] = err
}

func (e *Errs) get(method string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.m[method]
}

// Calls records method invocations for assertions.
type Calls struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *Calls) add(method string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n == nil {
		c.n = map[string]int{}
	}
	c.n[method]++
}

// Count returns how many times method was called.
func (c *Calls) Count(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n[method]
}

func ptr[T any](v T) *T { return &v }

// Ptr returns a pointer to v (handy for optional model fields).
func Ptr[T any](v T) *T { return ptr(v) }

// Logger is a no-op logger.Logger that records messages.
type Logger struct {
	mu   sync.Mutex
	Msgs []string
}

var _ logger.Logger = (*Logger)(nil)

func (l *Logger) rec(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Msgs = append(l.Msgs, level+": "+msg)
}

func (l *Logger) Debug(msg string, _ ...logger.Field)   { l.rec("debug", msg) }
func (l *Logger) Info(msg string, _ ...logger.Field)    { l.rec("info", msg) }
func (l *Logger) Success(msg string, _ ...logger.Field) { l.rec("success", msg) }
func (l *Logger) Warn(msg string, _ ...logger.Field)    { l.rec("warn", msg) }
func (l *Logger) Error(msg string, _ ...logger.Field)   { l.rec("error", msg) }
func (l *Logger) Fatal(msg string, _ ...logger.Field)   { l.rec("fatal", msg) }
func (l *Logger) Flush()                                {}

// Count returns how many recorded messages start with "level: ".
func (l *Logger) Count(level string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, m := range l.Msgs {
		if len(m) > len(level) && m[:len(level)+1] == level+":" {
			n++
		}
	}
	return n
}
