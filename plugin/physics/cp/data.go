// Package lottiecp is the static plugin for rigid-body collision data: the
// fixed silhouette (circles, boxes, convex polygons) an editor places over
// a character, stored in a dotLottie bundle under extensions/physics/cp/
// and wired straight into a jakecoffman/cp Space.
//
// The lottie-go core knows nothing about this payload — it only carries
// extensions/ files through a rewrite verbatim. Importing this package is
// what gives the data a schema, readers, and writers; a program that never
// imports it never links this code or the cp engine.
//
// Coordinates are animation coordinates (the space Animation.Size
// describes, y down) and pass into cp untouched. A game that runs cp with
// y-up gravity flips the gravity sign or transforms when drawing, exactly
// as it would for any screen-space physics.
package lottiecp

import (
	"encoding/json"
	"fmt"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// Dir is the bundle subtree this plugin claims; one JSON document per
// body, named by its id.
const Dir = "extensions/physics/cp/"

// Point is a 2D point in animation coordinates.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// BodyType selects how cp integrates the body.
type BodyType string

const (
	BodyDynamic   BodyType = "dynamic"
	BodyKinematic BodyType = "kinematic"
	BodyStatic    BodyType = "static"
)

// Body is one rigid body definition: the fixed collision silhouette of a
// character or prop. It lives at bundle level, not per animation — the
// same capsule serves idle and run alike, which is why it is not keyframed.
type Body struct {
	// Type defaults to dynamic when empty.
	Type BodyType `json:"type,omitempty"`
	// Mass applies to dynamic bodies; zero or negative reads as 1.
	Mass float64 `json:"mass,omitempty"`
	// Moment is the moment of inertia; zero means "derive it from the
	// shapes", which is what Build does.
	Moment float64 `json:"moment,omitempty"`
	Shapes []Shape `json:"shapes"`

	Extra lottie.ExtraFields `json:"-"`
}

func (b Body) MarshalJSON() ([]byte, error) {
	type alias Body
	return lottie.MarshalWithExtra(alias(b), b.Extra)
}

func (b *Body) UnmarshalJSON(data []byte) error {
	type alias Body
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*b = Body(a)
	b.Extra = extra
	return nil
}

// ShapeType names the cp shape a Shape builds.
type ShapeType string

const (
	ShapeCircle  ShapeType = "circle"
	ShapeBox     ShapeType = "box"
	ShapePolygon ShapeType = "polygon"
)

// Shape is one collision shape attached to a Body, in body-local animation
// coordinates.
type Shape struct {
	Type ShapeType `json:"type"`
	// Center places a circle or a box.
	Center Point `json:"center,omitzero"`
	// Radius is the circle's radius; for a box or polygon it is the corner
	// rounding cp applies.
	Radius float64 `json:"radius,omitempty"`
	// Width and Height size a box.
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	// Vertices are a convex polygon's corners.
	Vertices []Point `json:"vertices,omitempty"`

	Friction   float64 `json:"friction,omitempty"`
	Elasticity float64 `json:"elasticity,omitempty"`
	// Sensor shapes report contacts without colliding.
	Sensor bool `json:"sensor,omitempty"`

	Extra lottie.ExtraFields `json:"-"`
}

func (s Shape) MarshalJSON() ([]byte, error) {
	type alias Shape
	return lottie.MarshalWithExtra(alias(s), s.Extra)
}

func (s *Shape) UnmarshalJSON(data []byte) error {
	type alias Shape
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*s = Shape(a)
	s.Extra = extra
	return nil
}

// ParseBody decodes one body document.
func ParseBody(data []byte) (*Body, error) {
	var b Body
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("lottiecp: body: %w", err)
	}
	return &b, nil
}

// ---- bundle storage ----

func fileName(id string) string { return Dir + id + ".json" }

// IDs returns the body ids stored in the bundle, sorted.
func IDs(b *lottie.Bundle) []string {
	var out []string
	for _, name := range b.ExtensionFiles(Dir) {
		out = append(out, strings.TrimSuffix(strings.TrimPrefix(name, Dir), ".json"))
	}
	return out
}

// Load parses the body with the given id out of the bundle. Each call
// parses afresh; a caller editing in place keeps the pointer and Stores it
// back.
func Load(b *lottie.Bundle, id string) (*Body, error) {
	data, ok := b.ExtensionFile(fileName(id))
	if !ok {
		return nil, fmt.Errorf("lottiecp: no body %q in bundle", id)
	}
	body, err := ParseBody(data)
	if err != nil {
		return nil, fmt.Errorf("lottiecp: body %q: %w", id, err)
	}
	return body, nil
}

// Store writes a body into the bundle, where Encode carries it under
// extensions/physics/cp/ — with or without this plugin imported at that
// point.
func Store(b *lottie.Bundle, id string, body *Body) error {
	if id == "" {
		return fmt.Errorf("lottiecp: body id must not be empty")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("lottiecp: body %q: %w", id, err)
	}
	return b.SetExtensionFile(fileName(id), data)
}

// Remove drops a body from the bundle.
func Remove(b *lottie.Bundle, id string) {
	b.RemoveExtensionFile(fileName(id))
}
