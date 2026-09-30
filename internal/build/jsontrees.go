package build

import (
	"sync"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// jsonTrees keeps, per file name, the tree of the last JSON source a load parsed without error:
// its source file is the generation's, the same while the content is (cacheGen.file), and a tree
// is never changed once parsed, so a later run reads it again (NFR-02).
type jsonTrees struct {
	mu    sync.Mutex
	trees map[fileName]parsedJSON
	made  int // the trees parsed, for tests
}

// parsedJSON is a source file and its tree.
type parsedJSON struct {
	src  *source.File
	root *jsonsrc.Node
}

// parse is src's tree: the one kept for its name when it is the same file, else parsed and kept
// when it parses; a failure reports into bag as jsonsrc.Parse does, and keeps nothing.
func (j *jsonTrees) parse(src *source.File, bag *diag.Bag) (*jsonsrc.Node, error) {
	name := fileName{display: src.Path, abs: src.Abs}
	j.mu.Lock()
	kept, ok := j.trees[name]
	j.mu.Unlock()
	if ok && kept.src == src {
		return kept.root, nil
	}
	root, err := jsonsrc.Parse(src, bag)
	if err != nil {
		return nil, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.trees == nil {
		j.trees = map[fileName]parsedJSON{}
	}
	j.trees[name] = parsedJSON{src: src, root: root}
	j.made++
	return root, nil
}
