/*
 * JuiceFS, Copyright 2026 Juicedata, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"math"
	"strconv"
	"testing"
)

func TestParseGuidMode(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		want    string
		wantErr bool
	}{
		{"missing mode", "", guidModeLegacy, false},
		{"legacy mode", guidModeLegacy, guidModeLegacy, false},
		{"int31 mode", guidModeInt31, guidModeInt31, false},
		{"uint32 mode", guidModeUint32, guidModeUint32, false},
		{"invalid mode", "wide", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseGuidMode(c.mode)
			if (err != nil) != c.wantErr {
				t.Fatalf("parseGuidMode(%q) error = %v, wantErr %v", c.mode, err, c.wantErr)
			}
			if got != c.want {
				t.Fatalf("parseGuidMode(%q) = %q, want %q", c.mode, got, c.want)
			}
		})
	}
}

func TestGuidMask(t *testing.T) {
	cases := []struct {
		name     string
		mode     string
		metaName string
		want     uint32
	}{
		{"uint32 on mysql", guidModeUint32, "mysql", 0},
		{"uint32 on redis", guidModeUint32, "redis", 0},
		{"int31 on redis", guidModeInt31, "redis", 0x7FFFFFFF},
		{"int31 on sqlite", guidModeInt31, "sqlite3", 0x7FFFFFFF},
		{"legacy mysql", guidModeLegacy, "mysql", 0x7FFFFFFF},
		{"legacy postgres", guidModeLegacy, "postgres", 0x7FFFFFFF},
		{"legacy sqlite3", guidModeLegacy, "sqlite3", 0x7FFFFFFF},
		{"legacy redis", guidModeLegacy, "redis", 0},
		{"legacy tikv", guidModeLegacy, "tikv", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := guidMask(c.mode, c.metaName); got != c.want {
				t.Fatalf("guidMask(%q, %q) = %#x, want %#x", c.mode, c.metaName, got, c.want)
			}
		})
	}
}

// TestGenGuidRespectsMask makes sure the same volume+name yields a stable id and
// that the mask decides whether the full uint32 range is used. It also captures
// the cross-engine bug from issue #7338: a hash whose high bit is set differs
// between the uint32 and int31 modes.
func TestGenGuidRespectsMask(t *testing.T) {
	m := &mapping{salt: "uidcheck"}

	// Find a name whose raw uint32 hash exceeds MaxInt32 so int31 masking changes it.
	var name string
	var raw uint32
	for i := 0; i < 100000; i++ {
		n := "user" + strconv.Itoa(i)
		if id := m.genGuid(n); id > math.MaxInt32 {
			name, raw = n, id
			break
		}
	}
	if name == "" {
		t.Skip("no sample name with high-bit hash found")
	}

	m.mask = 0
	if got := m.genGuid(name); got != raw {
		t.Fatalf("uint32 mode is not deterministic: %d != %d", got, raw)
	}
	m.mask = 0x7FFFFFFF
	int31 := m.genGuid(name)
	if int31 > math.MaxInt32 {
		t.Fatalf("int31 mode produced out-of-range id: %d", int31)
	}
	if int31 == raw {
		t.Fatalf("expected int31 id to differ from uint32 id for high-bit hash")
	}
	if int31 != raw&0x7FFFFFFF {
		t.Fatalf("int31 id = %#x, want %#x", int31, raw&0x7FFFFFFF)
	}
}
