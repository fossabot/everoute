/*
Copyright 2021 The Lynx Authors.

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
	"fmt"
	"io"
	"os/exec"
	"regexp"
)

var (
	failureBegin = regexp.MustCompile(`^•! (Failure|Panic) in (Spec|Suite) (Setup|Teardown) \((Just)?(Before|After)(Suite|Each)\) \[\d*(\.\d*)? seconds]$`)
	failureEnd   = regexp.MustCompile(`^------------------------------$`)
	resultBegin  = regexp.MustCompile(`^(SUCCESS|FAIL)! -- \d* Passed \| \d* Failed \| \d* Pending \| \d* Skipped$`)
)

func startE2eRunner(logWriter io.Writer) (message string, failures []string, pass bool) {
	fmt.Fprintln(logWriter, "=======================\nstart new e2e runner\n=======================")

	var (
		resultMatcher  = &regexpWriter{beginMatcher: resultBegin}
		failureMatcher = &regexpWriter{beginMatcher: failureBegin, endMatcher: failureEnd}
	)

	runner := exec.Command("/usr/local/bin/e2e.test", "--test.timeout", "30m", "--test.v", "--ginkgo.noColor")
	runner.Stdout = io.MultiWriter(logWriter, resultMatcher, failureMatcher)
	runner.Stderr = logWriter
	err := runner.Run()

	if len(resultMatcher.result) != 0 {
		message = resultMatcher.result[0]
	} else if err != nil {
		message = err.Error()
	} else {
		message = "All checks have been passed"
	}

	return message, failureMatcher.result, err == nil
}

// regexpWriter save the lines matchs the regexp
type regexpWriter struct {
	// match the beginning line
	beginMatcher *regexp.Regexp
	// match the ending line
	endMatcher *regexp.Regexp

	matchEnd bool
	// buffer caches one line
	buffer []byte
	result []string
}

func (w *regexpWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			if !w.matchEnd {
				// should match begining line
				if w.beginMatcher.Match(w.buffer) {
					if w.endMatcher != nil {
						w.matchEnd = true
					}
					w.result = append(w.result, string(w.buffer))
				}
			} else {
				// should match ending line
				if w.endMatcher.Match(w.buffer) {
					w.matchEnd = false
				}
				w.result[len(w.result)-1] = fmt.Sprintf("%s\n%s", w.result[len(w.result)-1], string(w.buffer))
			}
			w.buffer = nil
		} else {
			w.buffer = append(w.buffer, b)
		}
	}
	return len(p), nil
}
