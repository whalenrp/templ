package templ_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/a-h/templ"
)

// Benchmark configuration
const (
	componentCount = 10 // Number of components to render per benchmark
)

// initializeContextAlwaysSyncMap creates a context that always uses sync.Map (concurrent mode).
//
// NOTE: To test the AlwaysSyncMap scenario, you need to temporarily modify runtime.go:
// In InitializeContext(), change:
//     mode: contextModeSingleThreaded,
// To:
//     mode: contextModeConcurrent,
//     ssConcurrent:          &sync.Map{},
//     onceHandlesConcurrent: &sync.Map{},
//
// Then rename all "Split" benchmarks to "AlwaysSyncMap" and re-run.
// This is the cleanest way to benchmark since contextValue internals are unexported.
func initializeContextAlwaysSyncMap(ctx context.Context) context.Context {
	return templ.InitializeContext(ctx)
}

// Component generators for different templ.Once usage percentages

// createComponents generates a mix of components based on oncePercentage (0, 10, 50, 100)
func createComponentsForSyncMapTest(oncePercentage int) []templ.Component {
	components := make([]templ.Component, componentCount)

	for i := 0; i < componentCount; i++ {
		useOnce := (i * 100 / componentCount) < oncePercentage

		if useOnce {
			// Component with templ.Once
			handle := templ.NewOnceHandle()
			components[i] = templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				// Render a script (uses shouldRenderScript)
				if err := writeScriptForTest(ctx, w, "test-script"); err != nil {
					return err
				}
				// Render a CSS class (uses shouldRenderClass)
				if err := writeClassForTest(ctx, w, "test-class"); err != nil {
					return err
				}
				// Render using Once (uses shouldRenderOnce)
				return handle.Once().Render(ctx, w)
			})
		} else {
			// Simple component without templ.Once
			components[i] = templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				// Still uses script and class deduplication
				if err := writeScriptForTest(ctx, w, "simple-script"); err != nil {
					return err
				}
				if err := writeClassForTest(ctx, w, "simple-class"); err != nil {
					return err
				}
				_, err := w.Write([]byte("<div>content</div>"))
				return err
			})
		}
	}

	return components
}

// createComponentsWithWork generates components with simulated work for concurrent benchmarks
func createComponentsWithWork(oncePercentage int, workDuration time.Duration) []templ.Component {
	components := make([]templ.Component, componentCount)

	for i := 0; i < componentCount; i++ {
		useOnce := (i * 100 / componentCount) < oncePercentage

		if useOnce {
			handle := templ.NewOnceHandle()
			components[i] = templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				// Simulate work (e.g., database lookup, API call)
				time.Sleep(workDuration)

				if err := writeScriptForTest(ctx, w, "test-script"); err != nil {
					return err
				}
				if err := writeClassForTest(ctx, w, "test-class"); err != nil {
					return err
				}
				return handle.Once().Render(ctx, w)
			})
		} else {
			components[i] = templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				// Simulate work
				time.Sleep(workDuration)

				if err := writeScriptForTest(ctx, w, "simple-script"); err != nil {
					return err
				}
				if err := writeClassForTest(ctx, w, "simple-class"); err != nil {
					return err
				}
				_, err := w.Write([]byte("<div>content</div>"))
				return err
			})
		}
	}

	return components
}

// Helper functions to render scripts and classes
func writeScriptForTest(ctx context.Context, w io.Writer, name string) error {
	script := templ.ComponentScript{
		Name:     name,
		Function: "function " + name + "() { console.log('test'); }",
	}
	return script.Render(ctx, w)
}

func writeClassForTest(ctx context.Context, w io.Writer, className string) error {
	css := templ.ComponentCSSClass{
		ID:    className,
		Class: templ.SafeCSS("." + className + " { color: red; }"),
	}
	return templ.RenderCSSItems(ctx, w, css)
}

// Sequential Benchmarks - 0% templ.Once usage

func BenchmarkSequential_0pct_Split(b *testing.B) {
	components := createComponentsForSyncMapTest(0)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		for _, comp := range components {
			if err := comp.Render(ctx, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// Sequential Benchmarks - 10% templ.Once usage

func BenchmarkSequential_10pct_Split(b *testing.B) {
	components := createComponentsForSyncMapTest(10)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		for _, comp := range components {
			if err := comp.Render(ctx, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// Sequential Benchmarks - 50% templ.Once usage

func BenchmarkSequential_50pct_Split(b *testing.B) {
	components := createComponentsForSyncMapTest(50)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		for _, comp := range components {
			if err := comp.Render(ctx, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// Sequential Benchmarks - 100% templ.Once usage

func BenchmarkSequential_100pct_Split(b *testing.B) {
	components := createComponentsForSyncMapTest(100)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		for _, comp := range components {
			if err := comp.Render(ctx, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// Concurrent Benchmarks - 0% templ.Once usage

func BenchmarkConcurrent_0pct_Split(b *testing.B) {
	// Use small work duration to make concurrency beneficial
	components := createComponentsWithWork(0, 100*time.Microsecond)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		concurrent := templ.Concurrent(components...)
		if err := concurrent.Render(ctx, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

// Concurrent Benchmarks - 10% templ.Once usage

func BenchmarkConcurrent_10pct_Split(b *testing.B) {
	components := createComponentsWithWork(10, 100*time.Microsecond)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		concurrent := templ.Concurrent(components...)
		if err := concurrent.Render(ctx, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

// Concurrent Benchmarks - 50% templ.Once usage

func BenchmarkConcurrent_50pct_Split(b *testing.B) {
	components := createComponentsWithWork(50, 100*time.Microsecond)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		concurrent := templ.Concurrent(components...)
		if err := concurrent.Render(ctx, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

// Concurrent Benchmarks - 100% templ.Once usage

func BenchmarkConcurrent_100pct_Split(b *testing.B) {
	components := createComponentsWithWork(100, 100*time.Microsecond)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		concurrent := templ.Concurrent(components...)
		if err := concurrent.Render(ctx, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

// Baseline: Sequential vs Concurrent comparison (no templ.Once)
// This helps understand pure concurrency overhead without deduplication

func BenchmarkBaseline_SequentialNoWork_0pct(b *testing.B) {
	components := createComponentsForSyncMapTest(0)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		for _, comp := range components {
			if err := comp.Render(ctx, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkBaseline_ConcurrentWithWork_0pct(b *testing.B) {
	components := createComponentsWithWork(0, 100*time.Microsecond)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := templ.InitializeContext(context.Background())
		concurrent := templ.Concurrent(components...)
		if err := concurrent.Render(ctx, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
