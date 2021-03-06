package model

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	log "github.com/cihub/seelog"
)

const (
	// MaxServiceLen the maximum length a service can have
	MaxServiceLen = 100
	// MaxNameLen the maximum length a name can have
	MaxNameLen = 100
	// MaxTypeLen the maximum length a span type can have
	MaxTypeLen = 100
	// MaxEndDateOffset the maximum amount of time in the future we
	// tolerate for span end dates
	MaxEndDateOffset = 10 * time.Minute
)

var (
	// Year2000NanosecTS is an arbitrary cutoff to spot weird-looking values
	Year2000NanosecTS = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()
)

// Normalize makes sure a Span is properly initialized and encloses the minimum required info
func (s *Span) Normalize() error {
	// Service
	if s.Service == "" {
		return errors.New("span.normalize: empty `Service`")
	}
	if len(s.Service) > MaxServiceLen {
		return fmt.Errorf("span.normalize: `Service` too long (max %d chars): %s", MaxServiceLen, s.Service)
	}
	// service shall comply with Datadog tag normalization as it's eventually a tag
	s.Service = NormalizeTag(s.Service)
	if s.Service == "" {
		return errors.New("span.normalize: `Service` could not be normalized")
	}

	// Name
	if s.Name == "" {
		return errors.New("span.normalize: empty `Name`")
	}
	if len(s.Name) > MaxNameLen {
		return fmt.Errorf("span.normalize: `Name` too long (max %d chars): %s", MaxNameLen, s.Name)
	}
	// name shall comply with Datadog metric name normalization
	var ok bool
	s.Name, ok = normMetricNameParse(s.Name)
	if !ok {
		return fmt.Errorf("span.normalize: invalid `Name`: %s", s.Name)
	}

	// Resource
	if s.Resource == "" {
		return errors.New("span.normalize: empty `Resource`")
	}

	// TraceID & SpanID should be set in the client
	// because they uniquely define the traces and associate them into traces
	if s.TraceID == 0 {
		return errors.New("span.normalize: empty `TraceID`")
	}
	if s.SpanID == 0 {
		return errors.New("span.normalize: empty `SpanID`")
	}

	// ParentID, TraceID and SpanID set in the client could be the same
	// Supporting the ParentID == TraceID == SpanID for the root span, is compliant
	// with the Zipkin implementation. Furthermore, as described in the PR
	// https://github.com/openzipkin/zipkin/pull/851 the constraint that the
	// root span's ``trace id = span id`` has been removed
	if s.ParentID == s.TraceID && s.ParentID == s.SpanID {
		s.ParentID = 0
		log.Debugf("span.normalize: `ParentID`, `TraceID` and `SpanID` are the same; `ParentID` set to 0: %d", s.TraceID)
	}

	// Start & Duration as nanoseconds timestamps
	// if s.Start is very little, less than year 2000 probably a unit issue so discard
	// (or it is "le bug de l'an 2000")
	if s.Start < Year2000NanosecTS {
		return fmt.Errorf("span.normalize: invalid `Start` (must be nanosecond epoch): %d", s.Start)
	}

	// If the end date is too far away in the future, it's probably a mistake.
	if s.Start+s.Duration > time.Now().Add(MaxEndDateOffset).UnixNano() {
		return fmt.Errorf("span.normalize: more than %v in the future", MaxEndDateOffset)
	}

	if s.Duration <= 0 {
		return errors.New("span.normalize: durations need to be strictly positive")
	}

	// ParentID set on the client side, no way of checking

	// Type
	if len(s.Type) > MaxTypeLen {
		return fmt.Errorf("span.normalize: `Type` too long (max %d chars): %s", MaxTypeLen, s.Type)
	}

	// Environment
	if env, ok := s.Meta["env"]; ok {
		s.Meta["env"] = NormalizeTag(env)
	}

	// Status Code
	if sc, ok := s.Meta["http.status_code"]; ok {
		if !isValidStatusCode(sc) {
			delete(s.Meta, "http.status_code")
			log.Debugf("Drop invalid meta `http.status_code`: %s", sc)
		}
	}

	return nil
}

// NormalizeTrace takes a trace and
// * rejects the trace if there is a trace ID discrepancy between 2 spans
// * rejects the trace if two spans have the same span_id
// * rejects empty traces
// * rejects traces where at least one span cannot be normalized
// * return the normalized trace and an error:
//   - nil if the trace can be accepted
//   - an error string if the trace needs to be dropped
func NormalizeTrace(t Trace) (Trace, error) {
	if len(t) == 0 {
		return t, errors.New("empty trace")
	}

	spanIDs := make(map[uint64]struct{})

	traceID := t[0].TraceID
	for i, span := range t {
		if _, ok := spanIDs[span.SpanID]; ok {
			return t, fmt.Errorf("duplicate span id %v (span %v)",
				span.SpanID, span)
		}

		if span.TraceID != traceID {
			return t, fmt.Errorf("trace id mismatch %s:%x != %s:%x",
				t[0].Name, t[0].TraceID, span.Name, span.TraceID)
		}

		if err := t[i].Normalize(); err != nil {
			return t, fmt.Errorf("invalid span %v: %v", span, err)
		}

		spanIDs[span.SpanID] = struct{}{}
	}

	return t, nil
}

func isValidStatusCode(sc string) bool {
	if code, err := strconv.ParseUint(sc, 10, 64); err == nil {
		return 100 <= code && code < 600
	}
	return false
}

// This code is borrowed from dd-go metric normalization

// fast isAlpha for ascii
func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// fast isAlphaNumeric for ascii
func isAlphaNum(b byte) bool {
	return isAlpha(b) || (b >= '0' && b <= '9')
}

// normMetricNameParse normalizes metric names with a parser instead of using
// garbage-creating string replacement routines.
func normMetricNameParse(name string) (string, bool) {
	if name == "" || len(name) > MaxNameLen {
		return name, false
	}

	var i, ptr int
	res := make([]byte, 0, len(name))

	// skip non-alphabetic characters
	for ; i < len(name) && !isAlpha(name[i]); i++ {
	}

	// if there were no alphabetic characters it wasn't valid
	if i == len(name) {
		return "", false
	}

	for ; i < len(name); i++ {
		switch {
		case isAlphaNum(name[i]):
			res = append(res, name[i])
			ptr++
		case name[i] == '.':
			// we skipped all non-alpha chars up front so we have seen at least one
			switch res[ptr-1] {
			// overwrite underscores that happen before periods
			case '_':
				res[ptr-1] = '.'
			default:
				res = append(res, '.')
				ptr++
			}
		default:
			// we skipped all non-alpha chars up front so we have seen at least one
			switch res[ptr-1] {
			// no double underscores, no underscores after periods
			case '.', '_':
			default:
				res = append(res, '_')
				ptr++
			}
		}
	}

	if res[ptr-1] == '_' {
		res = res[:ptr-1]
	}

	return string(res), true
}
