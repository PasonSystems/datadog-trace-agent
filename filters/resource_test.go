package filters

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	tests := []struct {
		filter      string
		resource    string
		expectation bool
	}{
		{"/foo/bar", "/foo/bar", false},
		{"/foo/b.r", "/foo/bar", false},
		{"/foo/.*", "/foo/bar", false},
		{"/foo/.*", "GET /foo/bar", false},
		{"/foo.*", "/foo/bar/asdf", false},
		{"/foo.*", "/foo/bar/asdf?othersuff=xyz&the_thing=rst", false},
		{"[0-9]+", "/abcde", true},
		{"[0-9]+", "/abcde123", false},
		{"\\(foobar\\)", "(foobar)", false},
		{"\\(foobar\\)", "(bar)", true},
		{"(GET|POST) /healthcheck", "GET /foobar", true},
		{"(GET|POST) /healthcheck", "GET /healthcheck", false},
		{"(GET|POST) /healthcheck", "POST /healthcheck", false},
		{"SELECT COUNT\\(\\*\\) FROM BAR", "SELECT COUNT(*) FROM BAR", false},
	}

	for _, test := range tests {
		span := newTestSpan(test.resource, test.resource)
		filter := newTestFilter([]string{test.filter})

		assert.Equal(t, test.expectation, filter.Keep(span))
	}
}

func TestFilterSearchReplace(t *testing.T) {
	tests := []struct {
		search      string
		replace     string
		resource    string
		expectation string
	}{
		{"foo", "FOO", "/foo/bar", "/FOO/bar"},
		{"FOO", "foo", "/foo/bar", "/foo/bar"},
		{"foo", "FOO", "/foo/bar/foo", "/FOO/bar/FOO"},
		{"(/foo/bar/).*", "${1}extra", "/foo/bar/foo", "/foo/bar/extra"},
		{"(/foo/bar/).*", "${1}extra", "/foo/bar/foo/bar", "/foo/bar/extra"},
		{"bar", "BAR", "/foo/bar/foo/bar", "/foo/BAR/foo/BAR"},
	}

	for _, test := range tests {
		trace := newTestTrace(test.resource, test.resource)
        this_filter := []string{test.search, test.replace}
		filter := newSearchReplaceTestFilter([][]string{this_filter})
        filter.ApplyRegex(trace)

        for i := range trace {
            span := trace[i]
		    assert.Equal(t, test.expectation, span.Meta["http.url"])
        }
        assert.True(t, len(trace) > 1)
	}
}

// a filter instantiated with malformed expressions should let anything pass
func TestRegexCompilationFailure(t *testing.T) {
	filter := newTestFilter([]string{"[123", "]123", "{6}"})

	for i := 0; i < 100; i++ {
		span := fixtures.RandomSpan()
		assert.True(t, filter.Keep(span))
	}
}

func TestRegexEscaping(t *testing.T) {
	span := newTestSpan("[123", "")

	filter := newTestFilter([]string{"[123"})
	assert.True(t, filter.Keep(span))

	filter = newTestFilter([]string{"\\[123"})
	assert.False(t, filter.Keep(span))
}

func TestMultipleEntries(t *testing.T) {
	filter := newTestFilter([]string{"ABC+", "W+"})

	span := newTestSpan("ABCCCC", "")
	assert.False(t, filter.Keep(span))

	span = newTestSpan("WWW", "")
	assert.False(t, filter.Keep(span))
}

func TestMultipleRegex(t *testing.T) {
    resource := "/match1/match2/remainder"
    trace := newTestTrace(resource, resource)
    filter1 := []string{"match2", "replace2"}
    filter2 := []string{"match1", "replace1"}
    filter := newSearchReplaceTestFilter([][]string{filter1, filter2})
    filter.ApplyRegex(trace)
    
    span := trace[0]
    assert.Equal(t, "/replace1/replace2/remainder", span.Meta["http.url"])
}

func newTestFilter(blacklist []string) Filter {
	c := config.NewDefaultAgentConfig()
	c.Ignore["resource"] = blacklist

	return newResourceFilter(c)
}

func newSearchReplaceTestFilter(search_replace [][]string) Filter {
	c := config.NewDefaultAgentConfig()
	c.Regex["resource"] = search_replace

	return newResourceFilter(c)
}

func newTestSpan(resource string, meta_http_url string) *model.Span {
	span := fixtures.RandomSpan()
	span.Resource = resource
    span.Meta["http.url"] = meta_http_url
	return span
}

func newTestTrace(resource string, meta_http_url string) model.Trace {
    // Trace with 3 levels and 3 traces per level
    trace := fixtures.RandomTrace(3, 3)
    for i := range trace {
        trace[i].Resource = resource
        trace[i].Meta["http.url"] = meta_http_url
    }

    return trace
}
