package main

// Viewer mode's file watcher. Polling rather than OS notifications: the
// watched set is a handful of files, editors and build tools replace
// files atomically in ways that confuse per-file watches, and polling
// needs no dependency. A change only triggers a reload once the stats
// hold still for a full interval, so a tool writing several files (a
// repack touching the bundle, an agent rewriting clips one by one) is
// picked up when it finishes, not mid-write.

import (
	"os"
	"time"
)

const watchInterval = 300 * time.Millisecond

type fileStamp struct {
	mod  int64
	size int64
}

// stampOf returns the zero stamp for a missing file, which is just
// another value that differs from the baseline: a file mid-replace reads
// as "changing" until it is back and stable.
func stampOf(path string) fileStamp {
	st, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{mod: st.ModTime().UnixNano(), size: st.Size()}
}

type watcher struct {
	model    *Model
	baseline map[string]fileStamp // as of the last (re)load
	prev     map[string]fileStamp // as of the previous poll
	next     time.Time
}

func newWatcher(m *Model) *watcher {
	w := &watcher{model: m}
	w.rebase()
	return w
}

func (w *watcher) poll() map[string]fileStamp {
	cur := make(map[string]fileStamp, len(w.model.Sources()))
	for _, p := range w.model.Sources() {
		cur[p] = stampOf(p)
	}
	return cur
}

func (w *watcher) rebase() {
	w.baseline = w.poll()
	w.prev = w.baseline
}

func sameKeys(a, b map[string]fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func equalStamps(a, b map[string]fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// tick is called every frame; it stats the sources at watchInterval and
// reloads once a change has settled.
func (w *watcher) tick() {
	now := time.Now()
	if now.Before(w.next) {
		return
	}
	w.next = now.Add(watchInterval)
	cur := w.poll()
	if !sameKeys(cur, w.baseline) {
		// The source set itself changed — an Open or Import inside the
		// editor, not a disk edit. Follow it instead of reloading.
		w.rebase()
		return
	}
	switch {
	case equalStamps(cur, w.baseline):
		// Nothing changed (or the source set was swapped by an Open,
		// which rebase below already followed).
		w.prev = cur
	case !equalStamps(cur, w.prev):
		// Still being written; wait for it to settle.
		w.prev = cur
	default:
		w.model.Reload()
		w.rebase()
	}
}
