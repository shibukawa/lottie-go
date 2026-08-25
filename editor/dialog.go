package main

import (
	"errors"
	"path/filepath"

	"github.com/ncruces/zenity"
)

// Native file dialogs block for as long as the user is looking at them, so
// they run on their own goroutine and the result is applied from Tick. The
// game loop keeps rendering meanwhile, and every Model field stays owned by
// the main goroutine.

type dialogKind int

const (
	dialogOpenBundle dialogKind = iota
	dialogSaveBundle
	dialogImportClips
)

type dialogResult struct {
	kind  dialogKind
	paths []string
	err   error
}

var (
	bundleFilters = zenity.FileFilters{
		{Name: "dotLottie bundle", Patterns: []string{"*.lottie"}, CaseFold: true},
		{Name: "Lottie JSON", Patterns: []string{"*.json"}, CaseFold: true},
	}
	clipFilters = zenity.FileFilters{
		{Name: "Lottie clip", Patterns: []string{"*.json", "*.lottie"}, CaseFold: true},
	}
)

// DialogOpen reports whether a dialog is on screen, so the UI can refuse to
// open a second one.
func (m *Model) DialogOpen() bool { return m.dialogOpen }

// start launches f on a goroutine and routes its result back to Tick.
func (m *Model) start(kind dialogKind, f func() ([]string, error)) {
	if m.dialogOpen {
		return
	}
	m.dialogOpen = true
	m.generation++
	ch := m.dialog
	go func() {
		paths, err := f()
		ch <- dialogResult{kind: kind, paths: paths, err: err}
	}()
}

// BrowseOpen asks for a bundle or a bare clip to open.
func (m *Model) BrowseOpen() {
	m.start(dialogOpenBundle, func() ([]string, error) {
		p, err := zenity.SelectFile(
			zenity.Title("Open bundle"),
			zenity.Filename(m.path),
			bundleFilters,
		)
		return []string{p}, err
	})
}

// BrowseSaveAs asks where to write the bundle.
func (m *Model) BrowseSaveAs() {
	name := m.path
	if name == "" {
		name = "character.lottie"
	}
	m.start(dialogSaveBundle, func() ([]string, error) {
		p, err := zenity.SelectFileSave(
			zenity.Title("Save bundle"),
			zenity.Filename(name),
			zenity.ConfirmOverwrite(),
			bundleFilters,
		)
		return []string{p}, err
	})
}

// BrowseImport asks for clips to add, allowing several at once.
func (m *Model) BrowseImport() {
	m.start(dialogImportClips, func() ([]string, error) {
		return zenity.SelectFileMultiple(
			zenity.Title("Import clips"),
			clipFilters,
		)
	})
}

// PumpDialogs applies a finished dialog. Call it once per tick.
func (m *Model) PumpDialogs() {
	select {
	case r := <-m.dialog:
		m.dialogOpen = false
		m.apply(r)
	default:
	}
}

func (m *Model) apply(r dialogResult) {
	if r.err != nil {
		// Cancelling is a normal outcome, not something to report as a
		// failure.
		if errors.Is(r.err, zenity.ErrCanceled) {
			m.generation++
			return
		}
		m.setStatus("dialog failed: %v", r.err)
		m.generation++
		return
	}
	switch r.kind {
	case dialogOpenBundle:
		if p := firstPath(r.paths); p != "" {
			m.Open(p)
		}
	case dialogSaveBundle:
		if p := firstPath(r.paths); p != "" {
			m.Save(withLottieExt(p))
		}
	case dialogImportClips:
		if len(r.paths) == 0 {
			m.generation++
			return
		}
		for _, p := range r.paths {
			m.ImportClip(p)
		}
		m.setStatus("imported %d clip(s)", len(r.paths))
		m.generation++
	}
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// withLottieExt defaults a typed-in save name to the bundle extension, since
// the file is always written as dotLottie v2.
func withLottieExt(p string) string {
	if filepath.Ext(p) == "" {
		return p + ".lottie"
	}
	return p
}
