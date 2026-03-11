package concurrentintegration

import (
	"context"
	_ "embed"
	"os"
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
	actual, diff, err := htmldiff.DiffCtx(ctx, component, expected)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		if err := os.WriteFile("actual-sync.html", []byte(actual), 0644); err != nil {
			t.Errorf("failed to write actual-sync.html: %v", err)
		}
		t.Error(diff)
	}
}

func TestConcurrent(t *testing.T) {
	component := renderConcurrent(testData)

	ctx := templ.InitializeContext(context.Background())
	actual, diff, err := htmldiff.DiffCtx(ctx, component, expected)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		if err := os.WriteFile("actual-concurrent.html", []byte(actual), 0644); err != nil {
			t.Errorf("failed to write actual-concurrent.html: %v", err)
		}
		t.Error(diff)
	}
}

func TestConcurrentSeq(t *testing.T) {
	component := renderConcurrentSeq(testData)

	ctx := templ.InitializeContext(context.Background())
	actual, diff, err := htmldiff.DiffCtx(ctx, component, expected)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		if err := os.WriteFile("actual-concurrent-seq.html", []byte(actual), 0644); err != nil {
			t.Errorf("failed to write actual-concurrent-seq.html: %v", err)
		}
		t.Error(diff)
	}
}
