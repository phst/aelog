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

import "log/slog"

// Additional logging levels corresponding to [Cloud Logging severities].
//
// [Cloud Logging severities]: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#logseverity
const (
	LevelCritical = LevelError + 4*(iota+1)
	LevelAlert
	LevelEmergency
	LevelNotice = (LevelInfo + LevelWarn) / 2
)

// Aliases for the standard levels, just for symmetry reasons.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

func severityForLevel(l slog.Level) string {
	switch {
	case l <= LevelDebug:
		return "DEBUG"
	case l <= LevelInfo:
		return "INFO"
	case l <= LevelNotice:
		return "NOTICE"
	case l <= LevelWarn:
		return "WARNING"
	case l <= LevelError:
		return "ERROR"
	case l <= LevelCritical:
		return "CRITICAL"
	case l <= LevelAlert:
		return "ALERT"
	default:
		return "EMERGENCY"
	}
}
