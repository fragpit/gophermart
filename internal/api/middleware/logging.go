package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"time"
)

func Log() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			var err error
			var bodyBytes []byte
			var decompressedBody []byte
			if r.Body != nil {
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					slog.Error(
						"error reading request body",
						slog.Any("error", err),
					)
					http.Error(
						w,
						http.StatusText(http.StatusInternalServerError),
						http.StatusInternalServerError,
					)
					return
				}
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				if r.Header.Get("Content-Encoding") == "gzip" &&
					len(bodyBytes) > 0 {
					gz, err := gzip.NewReader(
						bytes.NewReader(bodyBytes))
					if err == nil {
						decompressedBody, _ = io.ReadAll(gz)
						gz.Close()
					}
				}
			}

			logBody := string(bodyBytes)
			if len(decompressedBody) > 0 {
				logBody = string(decompressedBody)
			}

			slog.Debug("request details",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("query", r.URL.RawQuery),
				slog.String("user_agent", r.UserAgent()),
				slog.String("referer", r.Referer()),
				slog.Int("content_length", int(r.ContentLength)),
				slog.String("host", r.Host),
				slog.String("protocol", r.Proto),
				slog.Any("headers", r.Header),
				slog.String("body", logBody),
			)

			ww := &responseWriter{ResponseWriter: w, statusCode: 200}

			next.ServeHTTP(ww, r)

			slog.Info("request completed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.statusCode),
				slog.Int("resp_size", ww.size),
				slog.Duration("duration", time.Since(start)),
				slog.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size

	return size, err
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
