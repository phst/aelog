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
	"log/slog"
	"slices"
)

func optionalStrings(args ...string) []slog.Attr {
	n := len(args)
	if n == 0 {
		return nil
	}
	if n%2 != 0 {
		panic("odd number of arguments")
	}
	r := make([]slog.Attr, 0, n/2)
	for len(args) > 0 {
		k := args[0]
		v := args[1]
		if v != "" {
			r = append(r, slog.String(k, v))
		}
		args = args[2:]
	}
	return r
}

func groupValue(args ...string) slog.Value {
	return slog.GroupValue(optionalStrings(args...)...)
}

func customAttrs(r *slog.Record, attrs []slog.Attr, groups []string) []slog.Attr {
	n := r.NumAttrs()
	if len(attrs)+n == 0 {
		return nil
	}
	attrs = slices.Grow(slices.Clone(attrs), n)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != MessageKey {
			attrs = append(attrs, a)
		}
		return true
	})
	for _, g := range groups {
		attrs = []slog.Attr{attr(g, slog.GroupValue(attrs...))}
	}
	return attrs
}

func attr(key string, value slog.Value) slog.Attr {
	return slog.Attr{Key: key, Value: value}
}
