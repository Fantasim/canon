package canon_test

import (
	"context"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

func ExampleProject_Info() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	info, err := p.Info(context.Background())
	if err != nil {
		return
	}
	fmt.Println(info.Name != "")
	// Output: true
}

func ExampleProject_CheckWith() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.CheckWith(context.Background(), canon.CheckRequest{Packages: []string{"game.items"}, Lang: "en"})
	if err != nil {
		return
	}
	fmt.Println(res.HasErrors())
	// Output: false
}
