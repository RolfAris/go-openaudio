package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// ZapLogger returns an Echo middleware that logs HTTP requests using zap.
func ZapLogger(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			req := c.Request()
			res := c.Response()

			// Call the next handler
			err := next(c)

			// Log the request
			latency := time.Since(start)

			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.Int("status", res.Status),
				zap.Duration("latency", latency),
				zap.String("remote_ip", c.RealIP()),
				zap.String("user_agent", req.UserAgent()),
			}

			// Include error in logs only if it's not an HTTP error
			// (404s, etc. are already reflected in the status code)
			if err != nil {
				if _, ok := err.(*echo.HTTPError); !ok {
					fields = append(fields, zap.Error(err))
				}
			}

			// Log based on status code
			if res.Status >= 500 {
				logger.Error("server error", fields...)
			} else if res.Status >= 400 {
				logger.Warn("client error", fields...)
			} else {
				logger.Info("request", fields...)
			}

			return err
		}
	}
}
