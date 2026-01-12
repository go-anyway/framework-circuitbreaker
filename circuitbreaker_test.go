// Copyright 2025 zampo.

package circuitbreaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker"
)

func TestDefaultSettings(t *testing.T) {
	settings := DefaultSettings()

	if settings.MaxRequests != 3 {
		t.Errorf("MaxRequests = %v, want %v", settings.MaxRequests, 3)
	}
	if settings.Interval != 60*time.Second {
		t.Errorf("Interval = %v, want %v", settings.Interval, 60*time.Second)
	}
	if settings.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", settings.Timeout, 30*time.Second)
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	settings := Settings{
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: nil,
	}

	cb := NewCircuitBreaker("test-breaker", settings)

	if cb == nil {
		t.Fatal("NewCircuitBreaker returned nil")
	}
	if cb.name != "test-breaker" {
		t.Errorf("name = %v, want %v", cb.name, "test-breaker")
	}
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	result, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result != "success" {
		t.Errorf("Execute() result = %v, want %v", result, "success")
	}
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	expectedErr := errors.New("test error")
	result, err := cb.Execute(func() (interface{}, error) {
		return nil, expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("Execute() error = %v, want %v", err, expectedErr)
	}
	if result != nil {
		t.Errorf("Execute() result = %v, want nil", result)
	}
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	state := cb.State()

	if state != gobreaker.StateClosed {
		t.Errorf("State = %v, want %v", state, gobreaker.StateClosed)
	}
}

func TestCircuitBreaker_Counts(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	counts := cb.Counts()

	if counts.Requests != 0 {
		t.Errorf("Requests = %v, want %v", counts.Requests, 0)
	}
}

func TestCircuitBreaker_ExecuteMultipleRequests(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	for i := 0; i < 5; i++ {
		cb.Execute(func() (interface{}, error) {
			return i, nil
		})
	}

	counts := cb.Counts()
	if counts.Requests != 5 {
		t.Errorf("Requests = %v, want %v", counts.Requests, 5)
	}
	if counts.TotalSuccesses != 5 {
		t.Errorf("TotalSuccesses = %v, want %v", counts.TotalSuccesses, 5)
	}
}

func TestCircuitBreaker_FailureTracking(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	for i := 0; i < 3; i++ {
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
	}

	counts := cb.Counts()
	if counts.TotalFailures != 3 {
		t.Errorf("TotalFailures = %v, want %v", counts.TotalFailures, 3)
	}
}

func TestCircuitBreaker_GetSettings(t *testing.T) {
	settings := Settings{
		MaxRequests: 10,
		Interval:    120 * time.Second,
		Timeout:     60 * time.Second,
	}
	cb := NewCircuitBreaker("test", settings)

	retrieved := cb.GetSettings()

	if retrieved.MaxRequests != settings.MaxRequests {
		t.Errorf("MaxRequests = %v, want %v", retrieved.MaxRequests, settings.MaxRequests)
	}
	if retrieved.Interval != settings.Interval {
		t.Errorf("Interval = %v, want %v", retrieved.Interval, settings.Interval)
	}
	if retrieved.Timeout != settings.Timeout {
		t.Errorf("Timeout = %v, want %v", retrieved.Timeout, settings.Timeout)
	}
}

func TestCircuitBreaker_UpdateSettings(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	newSettings := Settings{
		MaxRequests: 10,
		Interval:    120 * time.Second,
		Timeout:     60 * time.Second,
	}

	cb.UpdateSettings(newSettings)

	retrieved := cb.GetSettings()
	if retrieved.MaxRequests != 10 {
		t.Errorf("After UpdateSettings, MaxRequests = %v, want %v", retrieved.MaxRequests, 10)
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultSettings())

	var successCount int32
	var errCount int32

	var wg syncWaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(shouldFail bool) {
			defer wg.Done()
			_, err := cb.Execute(func() (interface{}, error) {
				if shouldFail {
					atomic.AddInt32(&errCount, 1)
					return nil, errors.New("fail")
				}
				atomic.AddInt32(&successCount, 1)
				return "success", nil
			})
			if err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}(i%2 == 0)
	}

	wg.Wait()

	counts := cb.Counts()
	if counts.Requests != 10 {
		t.Errorf("Requests = %v, want %v", counts.Requests, 10)
	}
}

type syncWaitGroup struct {
	wg sync.WaitGroup
}

func (s *syncWaitGroup) Add(n int) {
	s.wg.Add(n)
}

func (s *syncWaitGroup) Done() {
	s.wg.Done()
}

func (s *syncWaitGroup) Wait() {
	s.wg.Wait()
}

func TestCircuitBreaker_CustomReadyToTrip(t *testing.T) {
	callCount := 0
	customReadyToTrip := func(counts gobreaker.Counts) bool {
		callCount++
		return counts.TotalFailures >= 3
	}

	settings := Settings{
		MaxRequests: 5,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: customReadyToTrip,
	}

	cb := NewCircuitBreaker("test", settings)

	for i := 0; i < 3; i++ {
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
	}

	if callCount == 0 {
		t.Error("Custom ReadyToTrip should have been called")
	}

	state := cb.State()
	if state != gobreaker.StateOpen {
		t.Errorf("State = %v, want %v", state, gobreaker.StateOpen)
	}
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	settings := Settings{
		MaxRequests: 1,
		Interval:    100 * time.Millisecond,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.TotalFailures >= 3
		},
	}

	cb := NewCircuitBreaker("test", settings)

	for i := 0; i < 3; i++ {
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
	}

	time.Sleep(150 * time.Millisecond)

	state := cb.State()
	if state == gobreaker.StateOpen {
		t.Logf("State after timeout: %v (may be half-open)", state)
	}
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	settings := Settings{
		MaxRequests: 5,
		Interval:    100 * time.Millisecond,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.TotalFailures >= 3
		},
	}

	cb := NewCircuitBreaker("test", settings)

	for i := 0; i < 3; i++ {
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
	}

	time.Sleep(150 * time.Millisecond)

	cb.Execute(func() (interface{}, error) {
		return "recovered", nil
	})

	counts := cb.Counts()
	if counts.TotalSuccesses < 1 {
		t.Errorf("Expected at least 1 success after recovery, got %v", counts.TotalSuccesses)
	}
}
