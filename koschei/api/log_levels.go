package main

import (
	"io"
	"regexp"
)

// Railway labels every stderr line "error" and every stdout line "info". The
// standard logger writes everything to stderr, so routine progress lines
// ("worker started", "cycle: candidates=300") were indistinguishable from real
// failures. levelRoutingWriter sends lines that carry a failure marker to
// stderr and everything else to stdout. log.Logger issues one Write per entry,
// so each entry is classified whole.
type levelRoutingWriter struct {
	out    io.Writer
	errOut io.Writer
}

var logZeroFailureCounter = regexp.MustCompile(`(?i)\b(?:critical|fatal|panic|error|failed|failure|halted|unavailable|refused|denied|cannot)\s*=\s*0(?:\.0+)?\b`)

var logFailureMarker = regexp.MustCompile(`(?i)\b(critical|fatal|panic|error|failed|failure|halted|unavailable|refused|denied|cannot|could not)\b`)

func logLineIsFailure(line []byte) bool {
	return logFailureMarker.Match(logZeroFailureCounter.ReplaceAll(line, nil))
}

func (w levelRoutingWriter) Write(p []byte) (int, error) {
	if logLineIsFailure(p) {
		return w.errOut.Write(p)
	}
	return w.out.Write(p)
}
