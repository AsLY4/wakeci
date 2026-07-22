package main

import (
	"regexp"
	"sort"
	"strings"
)

// secretsRegex matches exactly one `{{ secrets.NAME }}` reference at a time.
var secretsRegex = regexp.MustCompile(`{{\s*secrets\.([A-Za-z0-9_]+)\s*}}`)

const redactedSecret = "***REDACTED***"

// injectSecrets replaces every `{{ secrets.NAME }}` reference in str with the
// named secret's value (or the literal "<no value>" if NAME isn't
// configured, matching text/template's own missing-key rendering).
//
// This deliberately does its own targeted substitution instead of running
// str through text/template: executing it as a template against
// Config.secrets (the previous approach) let a single {{ secrets.X }}
// reference - required only to pass the "does this string use secrets"
// check - unlock arbitrary template actions elsewhere in the same string,
// including referencing a *different* secret than the one named, or a
// {{ range $k, $v := . }} that dumped every configured secret at once. A
// plain regex substitution can only ever resolve the one named key.
func injectSecrets(str string) string {
	return secretsRegex.ReplaceAllStringFunc(str, func(match string) string {
		key := secretsRegex.FindStringSubmatch(match)[1]
		if value, ok := Config.secrets[key]; ok {
			return value
		}
		return "<no value>"
	})
}

// redactSecrets redacts configured secret values from build log text. Output
// arrives one line at a time, so multiline secrets are redacted by their
// non-empty line components. Longer values are replaced first to avoid
// leaking the suffix of a secret that contains another secret as a prefix.
func redactSecrets(str string) string {
	values := make([]string, 0, len(Config.secrets))
	seen := make(map[string]struct{}, len(Config.secrets))
	for _, secret := range Config.secrets {
		for _, value := range strings.FieldsFunc(secret, func(r rune) bool {
			return r == '\r' || r == '\n'
		}) {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		return len(values[i]) > len(values[j])
	})
	for _, value := range values {
		str = strings.ReplaceAll(str, value, redactedSecret)
	}
	return str
}
