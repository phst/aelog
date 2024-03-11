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
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/phst/aelog"
)

func ExampleMiddleware() {
	// Suppress time and other noise.
	removeNoise := func(_ []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case aelog.TimeKey, "userAgent", "remoteIp", "protocol":
			return slog.Group("")
		default:
			return a
		}
	}
	log := slog.New(aelog.NewHandler(
		os.Stdout,
		&slog.HandlerOptions{ReplaceAttr: removeNoise},
		&aelog.Options{ProjectID: "test"},
	))

	handler := func(w http.ResponseWriter, r *http.Request) {
		log.InfoContext(r.Context(), "hi")
		io.WriteString(w, "ok")
	}
	srv := httptest.NewServer(aelog.Middleware(http.HandlerFunc(handler)))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		panic(err)
	}
	// https://cloud.google.com/trace/docs/setup#force-trace
	req.Header.Add("X-Cloud-Trace-Context", "abc/123;o=1")
	resp, err = srv.Client().Do(req)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	// Output:
	// {"severity":"INFO","message":"hi","httpRequest":{"requestMethod":"GET","requestUrl":"/"}}
	// {"severity":"INFO","message":"hi","httpRequest":{"requestMethod":"GET","requestUrl":"/"},"logging.googleapis.com/trace":"projects/test/traces/abc","logging.googleapis.com/spanId":"123"}
}

func TestMiddleware(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")

	buf := new(bytes.Buffer)
	log := slog.New(aelog.NewHandler(buf, nil, nil))

	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Logf("received request for URL %q", r.RequestURI)
		log.InfoContext(r.Context(), "received HTTP request")
		io.WriteString(w, "hello world")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/foo", handler)
	srv := httptest.NewServer(aelog.Middleware(mux))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	// https://cloud.google.com/trace/docs/setup#force-trace
	req.Header.Add("X-Cloud-Trace-Context", "123abc/456;o=1")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected HTTP status %q", resp.Status)
	}

	got := parseRecords(t, buf)
	want := []map[string]any{{
		"message": "received HTTP request",
		"httpRequest": map[string]any{
			"requestMethod": "GET",
			"requestUrl":    "/foo",
			"userAgent":     "Go-http-client/1.1",
			"protocol":      "HTTP/1.1",
		},
		"logging.googleapis.com/trace":  "projects/test-project/traces/123abc",
		"logging.googleapis.com/spanId": "456",
	}}
	if diff := cmp.Diff(
		got, want,
		ignoreFields("remoteIp", aelog.SeverityKey, aelog.TimeKey, aelog.SourceLocationKey),
	); diff != "" {
		t.Error("-got +want", diff)
	}
}
