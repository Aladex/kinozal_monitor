package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	logger "kinozaltv_monitor/logging"
)

var log = logger.New("http")

// HTTPLogger returns a custom logging middleware that integrates with the application logger
func HTTPLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Execute the request
			err := next(c)

			// Calculate latency
			latency := time.Since(start)

			// Get request and response data
			req := c.Request()
			res := c.Response()

			// Determine status code
			status := res.Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}

			// Build log fields
			fields := map[string]interface{}{
				"remote_ip":     c.RealIP(),
				"host":          req.Host,
				"method":        req.Method,
				"uri":           req.RequestURI,
				"user_agent":    req.UserAgent(),
				"status":        status,
				"latency":       latency.Nanoseconds(),
				"latency_human": latency.String(),
				"bytes_in":      req.ContentLength,
				"bytes_out":     res.Size,
			}

			// Add error information if present
			if err != nil {
				fields["error"] = err.Error()
			} else {
				fields["error"] = ""
			}

			// Choose log level based on status code
			message := "HTTP Request"
			if status >= 500 {
				log.ErrorNew(message, fields)
			} else if status >= 400 {
				log.Warn(message, fields)
			} else {
				log.InfoNew(message, fields)
			}

			return err
		}
	}
}
