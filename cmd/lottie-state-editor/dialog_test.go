package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ncruces/zenity"
)

// The dialog runs on a goroutine and its result is applied from Tick, so the
// plumbing is exercised by feeding the channel directly; a real dialog would
// block on a human.
func TestPumpAppliesOpenResult(t *testing.T) {
	dir := t.TempDir()
	src := NewModel()
	src.ImportClip(writeClip(t, dir, "a", 10, ""))
	src.NewMachine()
	src.AddState()
	out := filepath.Join(dir, "b.lottie")
	src.Save(out)

	m := NewModel()
	m.dialogOpen = true
	m.dialog <- dialogResult{kind: dialogOpenBundle, paths: []string{out}}
	m.PumpDialogs()

	if m.DialogOpen() {
		t.Error("dialog still marked open after its result was applied")
	}
	if m.Machine() == nil {
		t.Fatalf("bundle not loaded: %s", m.Status())
	}
	if m.Path() != out {
		t.Errorf("Path() = %q; want %q", m.Path(), out)
	}
}

func TestPumpAppliesImportResult(t *testing.T) {
	dir := t.TempDir()
	a := writeClip(t, dir, "a", 10, "")
	b := writeClip(t, dir, "b", 10, "")

	m := NewModel()
	m.dialogOpen = true
	m.dialog <- dialogResult{kind: dialogImportClips, paths: []string{a, b}}
	m.PumpDialogs()

	if got := m.AnimationIDs(); len(got) != 2 {
		t.Errorf("AnimationIDs() = %v; want both clips", got)
	}
}

// Cancelling is an ordinary outcome and must not surface as an error.
func TestPumpTreatsCancelAsNormal(t *testing.T) {
	m := NewModel()
	before := m.Status()
	m.dialogOpen = true
	m.dialog <- dialogResult{kind: dialogOpenBundle, err: zenity.ErrCanceled}
	m.PumpDialogs()

	if m.DialogOpen() {
		t.Error("a cancelled dialog stayed open")
	}
	if m.Status() != before {
		t.Errorf("Status() = %q; a cancel should not report a failure", m.Status())
	}
}

func TestPumpReportsRealFailures(t *testing.T) {
	m := NewModel()
	m.dialogOpen = true
	m.dialog <- dialogResult{kind: dialogOpenBundle, err: errors.New("no display")}
	m.PumpDialogs()
	if !strings.Contains(m.Status(), "dialog failed") {
		t.Errorf("Status() = %q; want a dialog failure", m.Status())
	}
}

// Pumping with nothing queued must be a no-op, since it runs every tick.
func TestPumpIsIdleWhenNoDialogRan(t *testing.T) {
	m := NewModel()
	gen, status := m.Generation(), m.Status()
	for range 3 {
		m.PumpDialogs()
	}
	if m.Generation() != gen || m.Status() != status {
		t.Error("pumping an empty queue changed the model")
	}
}

// Only one dialog may be in flight; a second request while one is open is
// dropped rather than queued behind it.
func TestSecondDialogIsRefusedWhileOneIsOpen(t *testing.T) {
	m := NewModel()
	started := 0
	m.start(dialogOpenBundle, func() ([]string, error) {
		started++
		return nil, zenity.ErrCanceled
	})
	m.start(dialogOpenBundle, func() ([]string, error) {
		started++
		return nil, zenity.ErrCanceled
	})
	if !m.DialogOpen() {
		t.Fatal("DialogOpen() = false while a dialog is in flight")
	}
	// Drain the one that did start.
	r := <-m.dialog
	m.dialogOpen = false
	m.apply(r)
	if started != 1 {
		t.Errorf("%d dialogs started; want 1", started)
	}
}

func TestSaveNameGetsBundleExtension(t *testing.T) {
	if got := withLottieExt("/tmp/hero"); got != "/tmp/hero.lottie" {
		t.Errorf("withLottieExt = %q; want /tmp/hero.lottie", got)
	}
	if got := withLottieExt("/tmp/hero.lottie"); got != "/tmp/hero.lottie" {
		t.Errorf("withLottieExt changed an explicit extension: %q", got)
	}
}

func TestPumpSaveWritesBundle(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a", 10, ""))
	m.NewMachine()
	m.AddState()

	// No extension typed: the file is still written as dotLottie.
	target := filepath.Join(dir, "hero")
	m.dialogOpen = true
	m.dialog <- dialogResult{kind: dialogSaveBundle, paths: []string{target}}
	m.PumpDialogs()

	if _, err := os.Stat(target + ".lottie"); err != nil {
		t.Fatalf("bundle not written: %v (%s)", err, m.Status())
	}
}

// The whole reason dialogs run on a goroutine: a native dialog blocks for as
// long as the user looks at it, and the game loop must keep drawing.
func TestStartDoesNotBlockTheCaller(t *testing.T) {
	m := NewModel()
	release := make(chan struct{})
	returned := make(chan struct{})

	go func() {
		m.start(dialogOpenBundle, func() ([]string, error) {
			<-release // stand in for a dialog waiting on a human
			return nil, zenity.ErrCanceled
		})
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("start blocked on the dialog instead of handing it to a goroutine")
	}

	// And nothing is applied until the dialog actually finishes.
	m.PumpDialogs()
	if !m.DialogOpen() {
		t.Error("dialog reported closed while it is still up")
	}
	close(release)
	<-m.dialog
}
