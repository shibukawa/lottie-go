# motioncomic — a raster motion-comic sample

The same storyboard as [examples/cutscene](../cutscene) — a pig late for
school, a squirrel in a hurry, a corner — retold as a shoujo-manga motion
comic: the panel-by-panel style of PSP-era games like Kurohyou and
Gravity Rush, where a camera glides over a comic page, panels slam in one
after another, and characters move just a little inside each frame.

Where the cutscene sample is all vector shapes, this one is all raster:
every layer is a WebP image (screentone dots, hand-wobbled panel borders,
lashed sparkly eyes, flowers, a hand-lettered「ドンッ!!」). 480x272,
30 fps, 10 seconds.

The art is drawn programmatically by the generator — no downloaded images
and no bundled fonts, so there is no third-party licensing to track. The
JSON embeds the WebP files as data URIs; the loose files sit under
`assets/` for inspection.

The JSON is embedded in the command here, so it plays from anywhere:

    go run github.com/shibukawa/lottie-go/examples/motioncomic@latest

Inside the repository, `go run ./examples/motioncomic`, or open the file
in the general player:

    go run ./examples/player examples/motioncomic/motioncomic.json

Regenerate art and animation with:

    go run ./examples/motioncomic/gen

The generator is its own module (see `gen/go.mod`): it needs a raster
drawing library and a WebP encoder, and the library module should not
inherit those dependencies.

## WebP note

lottie-go decodes image assets with Go's `image.Decode` and only
registers PNG itself. WebP works by registering the decoder in the
binary that embeds the animation:

    import _ "golang.org/x/image/webp"

Both this command and examples/player do so.

## Page layout

One 1000x920 virtual comic page, four panels, a camera null gliding
between them. Cuts are Lottie markers.

| marker | frames | panel |
| --- | --- | --- |
| `panel1-seg` | 0–62 | Pig sprints right; town bg and speed lines drift inside the panel. Slides in from the left. |
| `panel2-seg` | 62–124 | Squirrel sprints left through the park; petals fall. Slides in from the right. |
| `panel3-seg` | 124–188 | The corner: a slanted impact panel slams in with a flash, camera punch-in and shake,「ドンッ!!」pops over the border. |
| `panel4-seg` | 188–300 | Aftermath: dazed sway, orbiting twinkles, the toast lands, flowers everywhere. Fade to white. |

## How the "little movements" work

A precomp layer clips to its own width and height, so each panel interior
is a precomp: backgrounds drift, sprites bob and tremble inside it, and
nothing leaks past the border. The frame artwork — opaque paper outside,
transparent inside — lies on top and crops the slanted panel's corners.
The camera is a null the whole page parents to: its anchor tracks the
focus point, its scale leans in for the crash, and a few hold keyframes
are the impact shake.
