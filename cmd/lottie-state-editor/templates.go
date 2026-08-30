package main

// The New… dialog offers these bundled starting points. Templates are the
// presets themselves, embedded at build time; genpresets refreshes the
// copies under templates/ whenever it regenerates a preset.

import (
	"embed"
	"sort"
	"strings"
)

//go:embed templates/*.lottie
var templatesFS embed.FS

// templateNames lists the embedded templates, stably sorted.
func templateNames() []string {
	entries, err := templatesFS.ReadDir("templates")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".lottie"))
	}
	sort.Strings(names)
	return names
}

func templateBytes(name string) ([]byte, error) {
	return templatesFS.ReadFile("templates/" + name + ".lottie")
}
