package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter returns a Gin middleware that allows r tokens per second with burst b.
func RateLimiter(rps rate.Limit, burst int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rps, burst)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
