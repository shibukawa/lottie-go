package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The ebitenginedebug build tag makes Ebitengine dump one block per frame:
// a "----" separator, an "Internal image sizes:" census of every texture it
// holds, then "Graphics commands:" listing each command it actually sent to
// the driver after merging. Everything interesting about batching and texture
// growth is in there, but only in a form meant to be read by eye. summarizeLog
// reduces the last complete block to the handful of numbers worth tracking.
var (
	imageLine = regexp.MustCompile(`^ {2}(\d+): \((\d+), (\d+)\)(.*)$`)
	cmdLine   = regexp.MustCompile(`^ {2}([a-z][a-z-]*):`)
	dstField  = regexp.MustCompile(`dst: (\d+)`)
	idxField  = regexp.MustCompile(`num of indices: (\d+)`)
)

func summarizeLog(r io.Reader, w io.Writer) error {
	var current, last []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "----") {
			if hasCommands(current) {
				last = current
			}
			current = nil
			continue
		}
		current = append(current, line)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if hasCommands(current) {
		last = current
	}
	if last == nil {
		return fmt.Errorf("no frame with graphics commands found; was the log produced with -tags ebitenginedebug?")
	}

	var (
		images, bytes int
		screen        string
		draws         int
		indices       int
		perCommand    = map[string]int{}
		dsts          = map[string]bool{}
	)
	for _, line := range last {
		if m := imageLine.FindStringSubmatch(line); m != nil {
			iw, _ := strconv.Atoi(m[2])
			ih, _ := strconv.Atoi(m[3])
			images++
			bytes += iw * ih * 4
			if strings.Contains(m[4], "(screen)") {
				screen = fmt.Sprintf("%sx%s", m[2], m[3])
			}
			continue
		}
		if m := cmdLine.FindStringSubmatch(line); m != nil {
			draws++
			perCommand[m[1]]++
			if d := dstField.FindStringSubmatch(line); d != nil {
				dsts[d[1]] = true
			}
			if n := idxField.FindStringSubmatch(line); n != nil {
				v, _ := strconv.Atoi(n[1])
				indices += v
			}
		}
	}

	kinds := make([]string, 0, len(perCommand))
	for k := range perCommand {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s=%d", k, perCommand[k]))
	}

	fmt.Fprintf(w, "screen           : %s\n", screen)
	fmt.Fprintf(w, "commands         : %d (%s)\n", draws, strings.Join(parts, " "))
	fmt.Fprintf(w, "distinct targets : %d\n", len(dsts))
	fmt.Fprintf(w, "indices          : %d (%d triangles)\n", indices, indices/3)
	fmt.Fprintf(w, "internal textures: %d\n", images)
	fmt.Fprintf(w, "texture memory   : %.1f MiB\n", float64(bytes)/(1<<20))
	return nil
}

func hasCommands(block []string) bool {
	for _, line := range block {
		if strings.HasPrefix(line, "Graphics commands:") {
			return true
		}
	}
	return false
}
