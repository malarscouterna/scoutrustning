package i18n

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed messages/sv.json
var svJSON []byte

//go:embed messages/en.json
var enJSON []byte

var (
	once     sync.Once
	messages map[string]map[string]string
)

func load() {
	once.Do(func() {
		messages = make(map[string]map[string]string)
		var sv, en map[string]string
		json.Unmarshal(svJSON, &sv) //nolint:errcheck
		json.Unmarshal(enJSON, &en) //nolint:errcheck
		messages["sv"] = sv
		messages["en"] = en
	})
}

// T returns the translation for key in lang, substituting any {var} placeholders
// from vars. Falls back to Swedish if key is missing in the requested language,
// then to the key itself if missing entirely.
func T(lang, key string, vars ...map[string]string) string {
	load()
	val := messages[lang][key]
	if val == "" {
		val = messages["sv"][key]
	}
	if val == "" {
		return key
	}
	if len(vars) > 0 {
		for k, v := range vars[0] {
			val = strings.ReplaceAll(val, "{"+k+"}", v)
		}
	}
	return val
}

// Supported returns true if lang is a configured language.
func Supported(lang string) bool {
	load()
	_, ok := messages[lang]
	return ok
}
