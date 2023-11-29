// Copyright 2023 Google LLC
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
	ctx := context.WithValue(r.Context(), httpRequestKey, r)
	m.h.ServeHTTP(w, r.WithContext(ctx))
}

func httpAttrs(ctx context.Context, projectID string) []slog.Attr {
	req, ok := ctx.Value(httpRequestKey).(*http.Request)
	if !ok || req == nil {
		return nil
	}
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
	val := groupValue(
		"requestMethod", req.Method,
		"requestUrl", req.URL.String(),
		"userAgent", req.UserAgent(),
		"remoteIp", req.RemoteAddr,
		"referer", req.Referer(),
		"protocol", req.Proto,
	)
	attrs := []slog.Attr{attr("httpRequest", val)}
	trace, span := traceContext(req.Header, projectID)
	return append(attrs, optionalStrings("trace", trace, "spanId", span)...)
}

func traceContext(h http.Header, projectID string) (string, string) {
	if projectID == "" {
		// If we don’t have a project ID, we couldn’t format the trace
		// in the required format, so bail out.
		return "", ""
	}
	// https://cloud.google.com/trace/docs/setup#force-trace
	s, _, _ := strings.Cut(h.Get("X-Cloud-Trace-Context"), ";")
	trace, span, _ := strings.Cut(s, "/")
	if trace == "" {
		return "", ""
	}
	return fmt.Sprintf("projects/%s/traces/%s", projectID, trace), span
}

// See the comments for context.Context.Value.
type contextKey int

var httpRequestKey contextKey = 1
