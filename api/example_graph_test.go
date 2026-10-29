package canon_test

import (
	"context"
	"errors"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

func ExampleProject_Graph() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	kinds := map[canon.NodeKind]int{}
	var into []string
	err = p.Graph(context.Background(), func(it canon.GraphItem) error {
		switch {
		case it.Node != nil:
			kinds[it.Node.Kind]++
		case it.Edge.To == "teamboard:statuses.open":
			into = append(into, it.Edge.Pattern+" "+it.Edge.Path+" from "+it.Edge.From)
		}
		return nil
	})
	var pe *canon.PathError
	if errors.As(err, &pe) && errors.Is(err, canon.ErrNoValue) {
		fmt.Println("not computed:", pe.Path)
		return
	}
	if err != nil {
		return
	}
	fmt.Println(kinds[canon.NodeRoot] > 0, kinds[canon.NodeEntry] > 0, kinds[canon.NodeElement] > 0, kinds[canon.NodeMember] > 0, len(into))
	fmt.Println(into[0])
	// Output:
	// true true true true 6
	// teamboard:statuses[*].next[*] teamboard:statuses.taken.next[0] from teamboard:statuses.taken
}
