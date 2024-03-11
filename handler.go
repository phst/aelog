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
	"io"
	"log/slog"
	"os"
	"slices"
	"strconv"
)

// NewHandler creates a new [Handler].  The handler will write to the given
// [io.Writer] (typically [os.Stderr]).  It can be configured using generic
// [slog.HandlerOptions] and App Engine-specific [Options].  Passing nil for
// any of the options has the same effect as passing a pointer to a zero
// struct.
//
// If [Options] doesn’t contain a project ID, NewHandler attempts to
// auto-detect the current project; this typically works when running in
// production.  If no project can be detected, tracing information won’t be
// filled out.
func NewHandler(w io.Writer, basicOpts *slog.HandlerOptions, extOpts *Options) *Handler {
	if basicOpts == nil {
		basicOpts = new(slog.HandlerOptions)
	}
	if extOpts == nil {
		extOpts = new(Options)
	}
	repl := basicOpts.ReplaceAttr
	projectID := extOpts.ProjectID
	if projectID == "" {
		// https://cloud.google.com/appengine/docs/standard/go/runtime#environment_variables
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	jsonOpts := *basicOpts
	jsonOpts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		a = replaceAttr(groups, a)
		if repl != nil {
			a = repl(groups, a)
		}
		return a
	}
	return &Handler{
		base:      slog.NewJSONHandler(w, &jsonOpts),
		projectID: projectID,
	}
}

// Handler is an [slog.Handler] that sends structured log messages in JSON
// format.  Use [NewHandler] to create Handler objects; the zero Handler isn’t
// valid.  Handler objects can’t be copied once created.
type Handler struct {
	// We use an slog.JSONHandler because that does most of what we want.
	// We just need to munge the attributes a bit (in Handler.Handle and
	// replaceAttr).
	base *slog.JSONHandler

	// Empty only if we don’t know the project ID.
	projectID string

	// Attributes added by WithAttrs.
	attrs []slog.Attr

	// Names of groups added by Handler.WithGroup, from innermost to
	// outermost.
	groups []string
}

// Options contains additional options for configuring a [Handler].  It can be
// passed to [NewHandler].
type Options struct {
	// Alphanumeric Google Cloud project ID of the current project.  If
	// empty, NewHandler tries to auto-detect the project ID.
	ProjectID string
}

// Constants for [special keys] in the output record.
//
// [special keys]: https://cloud.google.com/logging/docs/structured-logging#special-payload-fields.
const (
	SeverityKey       = "severity"
	MessageKey        = "message"
	TimeKey           = "time"
	SourceLocationKey = "logging.googleapis.com/sourceLocation"
)

// Enabled implements [slog.Handler.Enabled].
func (h *Handler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.base.Enabled(ctx, l)
}

// Handle implements [slog.Handler.Handle].
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// See
	// https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
	// for a description of the fields that we set here.
	//
	// We try to optimize storage space by reusing the standard fields
	// (time, level, message, program counter) as much as possible.  The
	// slog.Record structure contains an optimization that stores a few
	// attributes inline.  By not using attributes for the standard fields
	// we can support that optimization a bit.  The replaceAttr function
	// will convert the attributes to the corresponding log record fields.
	s := slog.NewRecord(r.Time.UTC(), r.Level, r.Message, r.PC)
	s.AddAttrs(httpAttrs(ctx, h.projectID)...)
	if n := r.NumAttrs(); len(h.attrs)+n > 0 {
		attrs := slices.Grow(slices.Clone(h.attrs), n)
		r.Attrs(func(a slog.Attr) bool {
			if a.Key != MessageKey {
				attrs = append(attrs, a)
			}
			return true
		})
		for _, g := range h.groups {
			attrs = []slog.Attr{{Key: g, Value: slog.GroupValue(attrs...)}}
		}
		s.AddAttrs(attrs...)
	}
	return h.base.Handle(ctx, s)
}

// WithAttrs implements [slog.Handler.WithAttrs].
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	r := h.clone()
	r.attrs = append(r.attrs, attrs...)
	return r
}

// WithGroup implements [slog.Handler.WithGroup].
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	r := h.clone()
	r.groups = append([]string{name}, r.groups...)
	return r
}

func (h *Handler) clone() *Handler {
	r := *h
	r.attrs = slices.Clone(h.attrs)
	r.groups = slices.Clone(h.groups)
	return &r
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		// If we’re inside a group, don’t do anything.  Only top-level
		// attributes need munging.
		return a
	}
	v := a.Value
	// Translate standard attributes to the appropriate log record fields
	// and types.  We mostly leave unknown values intact.
	switch a.Key {
	case slog.TimeKey:
		a.Key = TimeKey
		// Handler.Handle has already converted the time to UTC.
	case slog.LevelKey:
		a.Key = SeverityKey
		if l, ok := v.Any().(slog.Level); ok {
			a.Value = slog.StringValue(severityForLevel(l))
		}
	case slog.MessageKey:
		a.Key = MessageKey
	case slog.SourceKey:
		a.Key = SourceLocationKey
		var s slog.Source
		// slog.Source is almost correct, but the
		// LogEntrySourceLocation record requires serializing the line
		// number as a string.
		if p, ok := v.Any().(*slog.Source); ok && p != nil {
			s = *p
		}
		attrs := make([]slog.Attr, 0, 3)
		if s.File != "" {
			attrs = append(attrs, slog.String("file", s.File))
		}
		if n := s.Line; n > 0 {
			// Don’t add a spurious line field to an
			// otherwise-empty group if we don’t have any line
			// information.
			attrs = append(attrs, slog.String("line", strconv.Itoa(n)))
		}
		if s.Function != "" {
			attrs = append(attrs, slog.String("function", s.Function))
		}
		a.Value = slog.GroupValue(attrs...)
	}
	return a
}
