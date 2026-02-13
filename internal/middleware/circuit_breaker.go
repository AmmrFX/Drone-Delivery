package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type circuitState int

const (
	stateClosed   circuitState = iota // normal operation
	stateOpen                         // rejecting requests
	stateHalfOpen                     // allowing one probe request
)

type circuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	failures         int
	threshold        int
	cooldown         time.Duration
	lastFailureTime  time.Time
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:     stateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return true
	case stateOpen:
		if time.Since(cb.lastFailureTime) > cb.cooldown {
			cb.state = stateHalfOpen
			return true
		}
		return false
	case stateHalfOpen:
		return false // only one probe at a time
	}
	return true
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = stateClosed
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = stateOpen
	}
}

// CircuitBreaker returns middleware that tracks failures per path and opens
// the circuit after `threshold` consecutive 5xx responses.
func CircuitBreaker(threshold int, cooldownSeconds int) gin.HandlerFunc {
	breakers := &sync.Map{}
	cooldown := time.Duration(cooldownSeconds) * time.Second

	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		val, _ := breakers.LoadOrStore(path, newCircuitBreaker(threshold, cooldown))
		cb := val.(*circuitBreaker)

		if !cb.allow() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{"code": "CIRCUIT_OPEN", "message": "service temporarily unavailable"},
			})
			return
		}

		c.Next()

		if c.Writer.Status() >= 500 {
			cb.recordFailure()
		} else {
			cb.recordSuccess()
		}
	}
}
