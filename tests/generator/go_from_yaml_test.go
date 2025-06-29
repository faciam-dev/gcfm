package generator_test

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/faciam-dev/gcfm/internal/generator"
)

func TestGenerateGoFromYAML(t *testing.T) {
	yamlData, err := os.ReadFile("../testdata/generator/registry.yaml")
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	got, err := generator.GenerateGoFromYAML(yamlData, generator.GoFromYAMLOptions{Package: "models", Table: "posts"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	want, err := os.ReadFile("../testdata/generator/cf_post.go")
	if err != nil {
		t.Fatalf("read want: %v", err)
	}
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}
