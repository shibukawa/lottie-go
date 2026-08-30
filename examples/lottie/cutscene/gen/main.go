// Command gen writes ../cutscene.json — the motion-comic sample described
// in the README next to it. Generated rather than downloaded so the
// licensing is unambiguous: everything is authored in this repository.
//
//	go run ./examples/lottie/cutscene/gen
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

func build() obj {
	layers := []obj{
		irisLayer(1),
		flashLayer(2),
		impactLayer(3),
		scatterLayer(4),
		exclaimLayer(5),
		sweatDropLayer(6),
		sweatRunLayer(7),
		precompLayer("pig-run-1", 8, "pig-comp", 200, 160, 0, c1End,
			tr(100, 150, 205, feetY, 90, 0)),
		precompLayer("squirrel-run-2", 9, "squirrel-comp", 160, 140, c1End, c2End,
			tr(78, 130, 275, feetY, 90, 0)),
		pigRun3Layer(10),
		squirrelRun3Layer(11),
		toastFallLayer(12),
		acornFallLayer(13),
		dizzyLayer("dizzy-pig", 14, 152, 130, 32, 900),
		dizzyLayer("dizzy-squirrel", 15, 346, 150, 25, -900),
		pigSitLayer(16),
		squirrelSitLayer(17),
		speedLayer("speed-1", 18, 0, c1End, 0, -1150),
		speedLayer("speed-2", 19, c1End, c2End, -1150, 0),
		shockLayer(20),
		dashesLayer("dashes-1", 21, 0, c1End, 0, -560),
		dashesLayer("dashes-2", 22, c1End, c2End, -460, 0),
		dashesLayer("dashes-3", 30, c2End, totalF, 0, 0),
		buildingsLayer(23),
		parkLayer(24),
		cornerLayer(25),
		roadLayer(26),
		cloudLayer("clouds-1", 27, 0, c1End, 0, -140),
		cloudLayer("clouds-2", 28, c1End, c2End, -100, 0),
		skyLayer(29),
	}
	markers := []obj{
		marker("cut1-seg", 0, c1End),
		marker("cut2-seg", c1End, c2End-c1End),
		marker("cut3-seg", c2End, c3End-c2End),
		marker("cut4-seg", c3End, totalF-c3End),
	}
	return doc("cutscene", stageW, stageH, totalF,
		[]obj{pigComp(), squirrelComp()}, layers, markers)
}

func run(out string) error {
	data, err := json.MarshalIndent(build(), "", " ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// The sample has to load clean in this repository's own player before
	// it is worth committing.
	anim, err := lottie.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("generated JSON does not decode: %w", err)
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) > 0 {
		return fmt.Errorf("generated JSON uses unsupported features: %s",
			strings.Join(unsup, ", "))
	}

	path := filepath.Join(out, "cutscene.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	w, h := anim.Size()
	fmt.Printf("wrote %s (%dx%d, %.1fs, %d bytes)\n",
		path, w, h, anim.Duration().Seconds(), len(data))
	return nil
}

func main() {
	out := flag.String("out", "examples/lottie/cutscene", "output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
