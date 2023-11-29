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

// Package aelog adds support for structured logging in Google App Engine apps.
//
// # Basic usage
//
// In the simplest case, create a new [Handler] and use it for the default
// logger:
//
//	func main() {
//		slog.SetDefault(slog.New(aelog.NewHandler(os.Stderr, nil, nil)))
//		// register handlers and start server
//	}
//
// Then, any logging through the package-level functions of the [slog] package
// will be formatted as JSON in a structure that the Google Cloud Logging
// machinery can parse.
//
// The handler maps logging levels to matching [severities].  Any unnamed level
// maps to the next-higher severity; for example, any level in the half-open
// interval ([slog.LevelWarn], [slog.LevelError]] maps to ERROR.  The package
// also defines a few additional named levels such as [LevelNotice].
//
// # HTTP middleware
//
// To support additional logging entries for HTTP requests, you can use the
// [Middleware] function to install an HTTP middleware.  For this to work, you
// need to wrap your HTTP handlers using [Middleware] and use the context-aware
// logging functions such as [slog.InfoContext] with a context returned by
// [http.Request.Context] (or a derived context).  See the example for the
// [Middleware] function for a worked-out example.
//
// [severities]: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#logseverity
package aelog
