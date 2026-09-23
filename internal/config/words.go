package config

import (
	"fmt"
	"strings"
)

// splitWords splits a config line into words the way a POSIX shell would:
// whitespace separates words, single quotes are literal, double quotes allow
// \" and \\ escapes and keep any other backslash (so "\t" reaches the
// command as backslash-t, as docker --format expects), and an unquoted
// backslash escapes the next character.
func splitWords(line string) ([]string, error) {
	var (
		words                  []string
		cur                    strings.Builder
		inWord, single, double bool
	)
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case single:
			if r == '\'' {
				single = false
			} else {
				cur.WriteRune(r)
			}
		case double:
			switch {
			case r == '"':
				double = false
			case r == '\\' && i+1 < len(rs) && (rs[i+1] == '"' || rs[i+1] == '\\'):
				i++
				cur.WriteRune(rs[i])
			default:
				cur.WriteRune(r)
			}
		case r == '\'':
			single, inWord = true, true
		case r == '"':
			double, inWord = true, true
		case r == '\\':
			if i+1 >= len(rs) {
				return nil, fmt.Errorf("trailing backslash")
			}
			i++
			cur.WriteRune(rs[i])
			inWord = true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if single || double {
		return nil, fmt.Errorf("unterminated quote")
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}
