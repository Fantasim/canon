package canon_test

import (
	"context"
	"testing"
)

// siteItems hold a @json(bits) list (C32) and an unrooted asset root (§12.3 `asset`).
const siteItems = `/// Items.
package items

/// Flag.
enum Flag @codes(UInt8) {
  A = 1
  B = 2
}

/// Item.
record Item {
  /// Icon.
  icon: asset("icons", ext: [png])
  /// Flags.
  flags: [Flag] = [] @json(bits)
}

/// Items.
let items: table Item = {
  sword { icon: "a.png", flags: [A] }
}
`

// siteShapes hold per-instance refs (J12) plain, optional and in a list, bits lists on a case
// field (C32), a literal union over a ref, and a map keyed by a ref.
const siteShapes = `/// Shapes.
package p

/// Flag.
enum Flag @codes(UInt8) {
  A = 1
  B = 2
}

/// Node.
record Node {
  /// Id.
  id: String
  /// Parent.
  parent: ref Node?
  /// Links.
  links: [ref Node] = []
}

/// Tree.
record Tree {
  /// Nodes.
  nodes: [Node] keyed by id
}

/// Shape.
variant Shape {
  /// Circle.
  circle {
    /// Flags.
    flags: [Flag] = [] @json(bits)
    /// Mode.
    mode: ref tags | "any" = "any"
    /// Optional flags.
    oflags: [Flag]? @json(bits)
  }
  /// Square.
  square
}

/// Tag.
record Tag {
  /// W.
  w: Int = 0
}

/// Tags.
let tags: table Tag = { red {}, blue {} }

/// Box.
record Box {
  /// Shape.
  shape: Shape
  /// Weights.
  weights: {ref tags: Int} = {}
  /// Tree.
  tree: Tree
}

/// Boxes.
let boxes: table Box = {
  one {
    shape: circle { flags: [A, B], mode: "blue", oflags: [B] }
    weights: { red: 1 }
    tree: { nodes: [ { id: "x" }, { id: "y", parent: "x", links: ["x"] } ] }
  }
}
`

// siteKinds are the sites and kinds the synthetic packages must reach.
var siteKinds = []string{
	"field:asset", "field:list", "field:optional", "field:union", "field:map", "elem:ref", "mapval:int", seenSib,
}

// API.md §5.2: TypeInfo.VM follows a field's encoding (C32), a ref's record (J12), an asset's root.
func TestValueTypeVMSites(t *testing.T) {
	fsys := newMemFS(map[string][]byte{
		"/law/project.canon": []byte(buildTestProject), "/law/items/item.canon": []byte(siteItems),
		"/law/items/icons/a.png": {}, "/law/p/p.canon": []byte(siteShapes),
	})
	p := openTierProject(t, fsys)
	if res, err := p.Check(context.Background()); err != nil || res.HasErrors() {
		t.Fatalf("Check: %v %+v", err, res)
	}
	w := newTypeWalk(t, p)
	w.roots("items")
	w.roots("p")
	for _, k := range siteKinds {
		if w.seen[k] == 0 {
			t.Errorf("no %s compared: %v", k, w.seen)
		}
	}
	for _, c := range []struct{ path, vm string }{
		{"items:items.sword.flags", `{"kind":"list","of":{"kind":"enum","ref":"items.Flag"},"unique":true}`},
		{"items:items.sword.icon", `{"kind":"asset","root":"items/icons","ext":["png"]}`},
		{"p:boxes.one.shape.flags", `{"kind":"list","of":{"kind":"enum","ref":"p.Flag"},"unique":true}`},
		{"p:boxes.one.shape.oflags", `{"kind":"optional","of":{"kind":"list","of":{"kind":"enum","ref":"p.Flag"},"unique":true}}`},
		{`p:boxes.one.tree.nodes["y"].parent`, `{"kind":"optional","of":{"kind":"ref","sibling":{"up":1,"field":"nodes"},"element":"p.Node","keyType":"string","count":0,"active":0}}`},
		{`p:boxes.one.tree.nodes["y"].links[#0]`, `{"kind":"ref","sibling":{"up":1,"field":"nodes"},"element":"p.Node","keyType":"string","count":0,"active":0}`},
	} {
		v, err := p.Value(context.Background(), c.path)
		if err != nil || string(v.Type.VM) != c.vm {
			t.Fatalf("%s: TypeInfo.VM = %v (%v), want %s", c.path, v, err, c.vm)
		}
	}
}
