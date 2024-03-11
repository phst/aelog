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

package aelog_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/phst/aelog"
)

func Example() {
	// This shows the most basic use case.
	slog.SetDefault(slog.New(aelog.NewHandler(os.Stderr, nil, nil)))
}

func ExampleHandler() {
	// Suppress timestamp noise.
	removeTime := func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == aelog.TimeKey {
			return slog.Group("")
		}
		return a
	}

	log := slog.New(aelog.NewHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: removeTime}, nil))

	log.Info("info")
	log.Warn("warning", "foo", "bar")
	log.Debug("this message won’t appear")
	log.With("foo", "bar").Info("info", "attr", 123)
	log.WithGroup("group").Error("error", "foo", "bar")

	// Output:
	// {"severity":"INFO","message":"info"}
	// {"severity":"WARNING","message":"warning","foo":"bar"}
	// {"severity":"INFO","message":"info","foo":"bar","attr":123}
	// {"severity":"ERROR","message":"error","group":{"foo":"bar"}}
}

func TestHandler_level(t *testing.T) {
	ctx := context.Background()

	buf := new(bytes.Buffer)
	log := slog.New(aelog.NewHandler(buf, nil, nil))

	log.Log(ctx, aelog.LevelWarn+1, "warning + 1")
	log.Warn("warning")
	log.Log(ctx, aelog.LevelNotice, "notice")
	log.Info("info")
	log.Debug("debug") // ignored

	got := parseRecords(t, buf)
	want := []map[string]any{
		{
			"severity": "ERROR",
			"message":  "warning + 1",
		},
		{
			"severity": "WARNING",
			"message":  "warning",
		},
		{
			"severity": "NOTICE",
			"message":  "notice",
		},
		{
			"severity": "INFO",
			"message":  "info",
		},
	}
	if diff := cmp.Diff(got, want, ignoreTime); diff != "" {
		t.Error("-got +want", diff)
	}
}

func TestHandler_attrs(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(aelog.NewHandler(buf, nil, nil))

	// Log a single warning with some attributes.
	log.Warn("test warning", "attr", 123, slog.Group("group", "inner", "value"))

	// This debug message shouldn’t show up.
	log.Debug("test debug message")

	got := parseRecords(t, buf)
	want := []map[string]any{{
		"severity": "WARNING",
		"message":  "test warning",
		"attr":     123.0,
		"group":    map[string]any{"inner": "value"},
	}}
	if diff := cmp.Diff(got, want, ignoreTime); diff != "" {
		t.Error("-got +want", diff)
	}
}

func TestHandler_time(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(aelog.NewHandler(buf, nil, nil))

	log.Info("message")

	got := parseRecords(t, buf)
	want := []map[string]any{{
		"severity": "INFO",
		"message":  "message",
		"time":     time.Now(),
	}}
	if diff := cmp.Diff(
		got, want,
		cmpopts.EquateApproxTime(time.Minute),
		cmp.FilterPath(isTime, cmp.Transformer("", parseTime)),
	); diff != "" {
		t.Error("-got +want", diff)
	}

	ts, ok := got[0]["time"].(string)
	if !ok {
		t.Error("time is not a string")
	}
	tm, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Error(err)
	}
	if tm.IsZero() {
		t.Error("zero time")
	}
	if tm.Location() != time.UTC {
		t.Error("time not in UTC")
	}
}

func TestHandler_source(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(aelog.NewHandler(buf, &slog.HandlerOptions{AddSource: true}, nil))

	// To determine whether source location is calculated correctly, call
	// runtime.Caller directly after logging.  These two statements have to
	// be directly adjacent to each other.
	log.Info("message")
	_, file, line, ok := runtime.Caller(0)
	if !ok {
		t.Error("couldn’t determine source location")
	}
	line-- // we want the line before

	got := parseRecords(t, buf)
	want := []map[string]any{{
		"severity": "INFO",
		"message":  "message",
		"logging.googleapis.com/sourceLocation": map[string]any{
			"file":     file,
			"line":     strconv.Itoa(line),
			"function": "github.com/phst/aelog_test.TestHandler_source",
		},
	}}
	if diff := cmp.Diff(got, want, ignoreTime); diff != "" {
		t.Error("-got +want", diff)
	}
}

func TestHandler_WithAttrs(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(aelog.NewHandler(buf, nil, nil))

	log.With("foo", "bar").Error("test error", "attr", 123)

	got := parseRecords(t, buf)
	want := []map[string]any{{
		"severity": "ERROR",
		"message":  "test error",
		"foo":      "bar",
		"attr":     123.0,
	}}
	if diff := cmp.Diff(got, want, ignoreTime); diff != "" {
		t.Error("-got +want", diff)
	}
}

func TestHandler_WithGroup(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(aelog.NewHandler(buf, nil, nil))

	log.WithGroup("outer").WithGroup("inner").Error("test error", "attr", 123)

	got := parseRecords(t, buf)
	want := []map[string]any{{
		"severity": "ERROR",
		"message":  "test error",
		"outer": map[string]any{
			"inner": map[string]any{"attr": 123.0},
		},
	}}
	if diff := cmp.Diff(got, want, ignoreTime); diff != "" {
		t.Error("-got +want", diff)
	}
}

func TestHandler_generic(t *testing.T) {
	buf := new(bytes.Buffer)
	err := slogtest.TestHandler(aelog.NewHandler(buf, nil, nil), func() []map[string]any { return parseRecords(t, buf) })
	if err != nil {
		t.Error(err)
	}
}

func parseRecords(t *testing.T, r io.Reader) (recs []map[string]any) {
	t.Helper()

	s := bufio.NewScanner(r)
	for s.Scan() {
		var m map[string]any
		if err := json.Unmarshal(s.Bytes(), &m); err != nil {
			t.Error(err)
		}
		recs = append(recs, m)
	}
	if err := s.Err(); err != nil {
		t.Error(err)
	}
	return
}

func ignoreFields(fields ...string) cmp.Option {
	ignore := func(key string, value any) bool {
		return slices.Contains(fields, key)
	}
	return cmpopts.IgnoreMapEntries(ignore)
}

var ignoreTime = ignoreFields(aelog.TimeKey)

func isTime(p cmp.Path) bool {
	if len(p) != 3 {
		return false
	}
	i, ok := p[2].(cmp.MapIndex)
	if !ok {
		return false
	}
	k := i.Key()
	return k.Kind() == reflect.String && k.String() == aelog.TimeKey
}

func parseTime(v any) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	s, ok := v.(string)
	t, err := time.Parse(time.RFC3339Nano, s)
	if !ok || err != nil {
		return time.Time{}
	}
	return t
}
