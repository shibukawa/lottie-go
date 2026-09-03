// Command lottieforge turns generated character art into a lottie-go
// character: a textured vector rig that inherits every clip of a preset
// (see skills/lottie-character-forge and .knowledge
// requirement:ai-character-forge).
//
//	lottieforge grid  work   # spec -> sheets.json, template PNGs, prompts
//	lottieforge cut   work   # sheets -> parts/, report.json, contact.png
//	lottieforge rig   work   # parts + base preset -> work/<name>.lottie
//	lottieforge morph work   # bake morph tracks and attachment motion
//
// work/ holds character.json (the spec) and everything derived from it.
package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage: lottieforge <grid|cut|rig|morph> [flags] [workdir]

  grid   write sheets.json, sheets/*.template.png and prompts/*.md from character.json
  cut    cut sheets/*.png into parts/ by sheets.json; write report.json and contact.png
  rig    build <name>.lottie from parts/ and the base preset
  morph  bake the spec's morph list and the attachments' motion into the bundle

Each subcommand takes -h for its flags. The work directory defaults to ".".`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "grid":
		err = runGrid(os.Args[2:])
	case "cut":
		err = runCut(os.Args[2:])
	case "rig":
		err = runRig(os.Args[2:])
	case "morph":
		err = runMorph(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lottieforge:", err)
		os.Exit(1)
	}
}
