package generator_test

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/faciam-dev/gcfm/pkg/registry/codec"
	"github.com/faciam-dev/gcfm/internal/generator"
)

func TestGenerateYAMLFromGo(t *testing.T) {
	src := "../testdata/generator/cf_post.go"
	gotYAML, err := generator.GenerateYAMLFromGo(generator.YAMLFromGoOptions{Srcs: []string{src}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	wantData, err := os.ReadFile("../testdata/generator/registry.yaml")
	if err != nil {
		t.Fatalf("read want: %v", err)
	}
	want, err := codec.DecodeYAML(wantData)
	if err != nil {
		t.Fatalf("decode want: %v", err)
	}
	got, err := codec.DecodeYAML(gotYAML)
	if err != nil {
		t.Fatalf("decode got: %v", err)
	}
	for i := range want {
		want[i].Display = nil
	}
	for i := range got {
		got[i].Display = nil
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}
