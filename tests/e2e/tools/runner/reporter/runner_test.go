/*
Copyright 2021 The Everoute Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"testing"
)

func TestRegexpWriter(t *testing.T) {
	testCases := []struct {
		beginMatcher *regexp.Regexp
		endMatcher   *regexp.Regexp
		input        string
		result       []string
	}{
		{
			beginMatcher: regexp.MustCompile(`^a$`),
			endMatcher:   regexp.MustCompile(`^c$`),
			input:        "a\nb\nc\nd\ne\n",
			result:       []string{"a\nb\nc"},
		},
		{
			beginMatcher: regexp.MustCompile(`^a$`),
			input:        "a\nb\nc\nd\ne\n",
			result:       []string{"a"},
		},
		{
			beginMatcher: regexp.MustCompile(`^a|d$`),
			input:        "a\nb\nc\nd\ne\n",
			result:       []string{"a", "d"},
		},
		{
			beginMatcher: regexp.MustCompile(`^a|d$`),
			endMatcher:   regexp.MustCompile(`^c|e$`),
			input:        "a\nb\nc\nd\ne\n",
			result:       []string{"a\nb\nc", "d\ne"},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test match writer %d", i), func(t *testing.T) {
			regexpWriter := &regexpWriter{
				beginMatcher: tc.beginMatcher,
				endMatcher:   tc.endMatcher,
			}
			_, err := io.Copy(regexpWriter, bytes.NewBufferString(tc.input))
			if err != nil {
				t.Fatalf("copy to writer: %s", err)
			}
			if !reflect.DeepEqual(tc.result, regexpWriter.result) {
				t.Fatalf("expect result: %v, actual: %v", tc.result, regexpWriter.result)
			}
		})
	}
}

func TestMatchFailureBegin(t *testing.T) {
	testCases := []string{
		"•! Panic in Spec Setup (BeforeEach) [0.000 seconds]",
		"•! Failure in Spec Setup (BeforeEach) [80.290 seconds]",
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("match failure begin %s", tc), func(t *testing.T) {
			if !failureBegin.MatchString(tc) {
				t.Fatalf("unexpected unmatch result %s", tc)
			}
		})
	}
}
