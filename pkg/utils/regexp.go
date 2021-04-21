package utils

import "regexp"

func MatchGroups(r *regexp.Regexp, str string) map[string]string {
	matched := r.FindStringSubmatch(str)
	results := make(map[string]string)
	names := r.SubexpNames()
	for i, value := range matched {
		results[names[i]] = value
	}
	return results
}
