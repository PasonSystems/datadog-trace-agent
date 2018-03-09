package filters

import (
	"regexp"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"

	log "github.com/cihub/seelog"
)

// ResourceFilter implements a resource-based filter
type resourceFilter struct {
	blacklist []*regexp.Regexp
    searchReplace []map[*regexp.Regexp]string
}

// Keep returns true if Span.Resource doesn't match any of the filter's rules
func (f *resourceFilter) Keep(t *model.Span) bool {
	for _, entry := range f.blacklist {
		if entry.MatchString(t.Resource) {
			return false
		}
	}

    for _, entry := range f.searchReplace {
	    for key, value := range entry {
	    	if key.MatchString(t.Meta["http.url"]) {
                t.Meta["http.url"] = key.ReplaceAllString(t.Meta["http.url"], value)
	    	}
	    }
    }

	return true
}

func newResourceFilter(conf *config.AgentConfig) Filter {
	blacklist := compileRules(conf.Ignore["resource"])
    searchReplace := compileSearchReplace(conf.Regex["resource"])

	return &resourceFilter{blacklist, searchReplace}
}

func compileRules(entries []string) []*regexp.Regexp {
	blacklist := make([]*regexp.Regexp, 0, len(entries))

	for _, entry := range entries {
		rule, err := regexp.Compile(entry)

		if err != nil {
			log.Errorf("invalid resource filter: %q", entry)
			continue
		}

		blacklist = append(blacklist, rule)
	}

	return blacklist
}

func compileSearchReplace(entries [][]string) []map[*regexp.Regexp]string {
	searchReplace := make([]map[*regexp.Regexp]string, 0, len(entries))

	for _, entry := range entries {
        if len(entry) != 2 {
            log.Errorf("Search/Replace entry invalid: %q", entry)
            continue
        }

        search, err := regexp.Compile(entry[0])

		if err != nil {
			log.Errorf("Unable to compile Search/Replace regex: %q", entry[0])
			continue
		}

        rule := make(map[*regexp.Regexp]string)
        rule[search] = entry[1]
		searchReplace = append(searchReplace, rule)
	}

    return searchReplace
}
