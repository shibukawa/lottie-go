// Package lottiespine imports Spine skeletons (the JSON export of Spine
// 4.x, with its texture atlas) into dotLottie bundles that play through
// lottie-go's texture extension.
//
// Nothing in Lottie is a bone or a skinned mesh, so the importer bakes: it
// evaluates the skeleton — bone hierarchy with every inherit mode, IK and
// transform constraints, deform keys, slot colors and attachment changes —
// at every frame of every animation and writes what it sees. Each
// animation becomes one clip. Each slot becomes a shape layer, each region
// or mesh attachment a group holding keyframed paths and a fill, and the
// texture document beside the clip (plugin/texture) paints the atlas page
// through those paths with the mesh's own UV per vertex. A mesh therefore
// keeps deforming with its art, which is what the extension's per-vertex
// mapping is for; a player without the extension sees the same shapes in
// their slot colors.
//
// Spine events become markers, slot blends become layer blend modes, and
// a generated state machine gives the game one event per animation. Path
// and physics constraints, clipping, draw-order keys and sequences are
// reported in Result.Notes rather than converted.
//
//	sk, _ := lottiespine.Parse(skeletonJSON)
//	atlas, _ := lottiespine.ParseAtlas(atlasText)
//	res, _ := lottiespine.Convert(sk, lottiespine.Options{Atlas: atlas, ReadPage: readPage})
//	b, _ := res.Bundle()
//
// cmd/lottierepack wraps this as -import-spine.
package lottiespine
