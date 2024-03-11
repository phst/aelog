// Copyright 2023, 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aelog

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// Middleware returns a derived version of the given HTTP handler that calls it
// after ensuring that a [Handler] can extract HTTP-specific information from
// HTTP requests.
func Middleware(h http.Handler) http.Handler {
	return &middleware{h}
}

type middleware struct{ h http.Handler }

func (m *middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
	val := slog.GroupValue(optionalStrings(
		"requestMethod", r.Method,
		"requestUrl", r.URL.String(),
		"userAgent", r.UserAgent(),
		"remoteIp", r.RemoteAddr,
		"referer", r.Referer(),
		"protocol", r.Proto,
	)...)
	trace, span := traceContext(r.Header)
	ctx := context.WithValue(r.Context(), httpInfoKey, &httpInfo{val, trace, span})
	m.h.ServeHTTP(w, r.WithContext(ctx))
}

func httpAttrs(ctx context.Context, projectID string) []slog.Attr {
	i, ok := ctx.Value(httpInfoKey).(*httpInfo)
	if !ok || i == nil {
		return nil
	}
	attrs := []slog.Attr{{Key: "httpRequest", Value: i.req}}
	return append(attrs, traceAttrs(projectID, i.trace, i.span)...)
}

func traceContext(h http.Header) (string, string) {
	// https://cloud.google.com/trace/docs/setup#force-trace
	s, _, _ := strings.Cut(h.Get("X-Cloud-Trace-Context"), ";")
	trace, span, _ := strings.Cut(s, "/")
	return trace, span
}

func traceAttrs(projectID, trace, span string) []slog.Attr {
	if projectID == "" || trace == "" {
		// If we don’t have a project ID, we couldn’t format the trace
		// in the required format, so bail out.
		return nil
	}
	return optionalStrings(
		"logging.googleapis.com/trace", fmt.Sprintf("projects/%s/traces/%s", projectID, trace),
		"logging.googleapis.com/spanId", span,
	)
}

type httpInfo struct {
	req         slog.Value
	trace, span string
}

// See the comments for context.Context.Value.
type contextKey int

const httpInfoKey contextKey = 1
