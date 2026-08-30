package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ncruces/zenity"
)

// Native file dialogs block for as long as the user is looking at them, so
// they run on their own goroutine and the result is applied from Tick,
// exactly as in the state machine editor.

type dialogKind int

const (
	dialogOpenScene dialogKind = iota
	dialogSaveScene
	dialogAddBundle
	dialogAddImage
	dialogAddFont
)

type dialogResult struct {
	kind  dialogKind
	paths []string
	err   error
}

var (
	sceneFilters = zenity.FileFilters{
		{Name: "Scene", Patterns: []string{"*.scene.json", "*.json"}, CaseFold: true},
	}
	bundleFilters = zenity.FileFilters{
		{Name: "dotLottie bundle", Patterns: []string{"*.lottie"}, CaseFold: true},
	}
	imageFilters = zenity.FileFilters{
		{Name: "Image", Patterns: []string{"*.png", "*.jpg", "*.jpeg", "*.webp"}, CaseFold: true},
	}
	fontFilters = zenity.FileFilters{
		{Name: "Font", Patterns: []string{"*.ttf", "*.otf"}, CaseFold: true},
	}
)

// DialogOpen reports whether a dialog is on screen, so the UI can refuse
// to open a second one.
func (m *Model) DialogOpen() bool { return m.dialogOpen }

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

// BrowseOpen asks for a scene file to open.
func (m *Model) BrowseOpen() {
	m.start(dialogOpenScene, func() ([]string, error) {
		p, err := zenity.SelectFile(
			zenity.Title("Open scene"),
			zenity.Filename(m.path),
			sceneFilters,
		)
		return []string{p}, err
	})
}

// BrowseSaveAs asks where to write the scene.
func (m *Model) BrowseSaveAs() {
	name := m.path
	if name == "" {
		name = "menu.scene.json"
	}
	m.start(dialogSaveScene, func() ([]string, error) {
		p, err := zenity.SelectFileSave(
			zenity.Title("Save scene"),
			zenity.Filename(name),
			zenity.ConfirmOverwrite(),
			sceneFilters,
		)
		return []string{p}, err
	})
}

// BrowseAddBundle asks for bundles to reference, allowing several at once.
func (m *Model) BrowseAddBundle() {
	m.start(dialogAddBundle, func() ([]string, error) {
		return zenity.SelectFileMultiple(
			zenity.Title("Add bundles"),
			bundleFilters,
		)
	})
}

// BrowseAddImage asks for image files to reference.
func (m *Model) BrowseAddImage() {
	m.start(dialogAddImage, func() ([]string, error) {
		return zenity.SelectFileMultiple(zenity.Title("Add images"), imageFilters)
	})
}

// BrowseAddFont asks for font files to reference.
func (m *Model) BrowseAddFont() {
	m.start(dialogAddFont, func() ([]string, error) {
		return zenity.SelectFileMultiple(zenity.Title("Add fonts"), fontFilters)
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
		if errors.Is(r.err, zenity.ErrCanceled) {
			m.generation++
			return
		}
		m.setStatus("dialog failed: %v", r.err)
		m.generation++
		return
	}
	switch r.kind {
	case dialogOpenScene:
		if p := firstPath(r.paths); p != "" {
			m.Open(p)
		}
	case dialogSaveScene:
		if p := firstPath(r.paths); p != "" {
			m.Save(withSceneExt(p))
		}
	case dialogAddBundle:
		for _, p := range r.paths {
			m.AddBundle(p)
		}
		if len(r.paths) == 0 {
			m.generation++
		}
	case dialogAddImage:
		for _, p := range r.paths {
			m.AddImage(p)
		}
		if len(r.paths) == 0 {
			m.generation++
		}
	case dialogAddFont:
		for _, p := range r.paths {
			m.AddFont(p)
		}
		if len(r.paths) == 0 {
			m.generation++
		}
	}
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// withSceneExt defaults a typed-in save name to the scene extension.
func withSceneExt(p string) string {
	if strings.HasSuffix(strings.ToLower(p), ".json") {
		return p
	}
	if filepath.Ext(p) == "" {
		return p + ".scene.json"
	}
	return p
}
