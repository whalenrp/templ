package concurrentintegration

import (
	"context"
	_ "embed"
	"testing"

	"github.com/a-h/templ"
	"github.com/a-h/templ/generator/htmldiff"
)

//go:embed expected.html
var expected string

var testData = []string{"First", "Second", "Third"}

func TestSync(t *testing.T) {
	component := renderSync(testData)

	ctx := templ.InitializeContext(context.Background())
	_, diff, err := htmldiff.DiffCtx(ctx, component, expected)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Error(diff)
	}
}

func TestConcurrentSeq(t *testing.T) {
	component := renderConcurrentSeq(testData)

	ctx := templ.InitializeContext(context.Background())
	_, diff, err := htmldiff.DiffCtx(ctx, component, expected)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Error(diff)
	}
}
