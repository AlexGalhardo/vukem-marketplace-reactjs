package tests

import (
	"encoding/json"
	"io/fs"
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/resources"
)

// TestCatalogsHaveSameKeys keeps en.json and pt-BR.json in sync: the React
// pages import both, so a missing key would render the raw key.
func TestCatalogsHaveSameKeys(t *testing.T) {
	t.Parallel()

	keys := func(name string) []string {
		b, err := fs.ReadFile(resources.Lang(), name)
		if err != nil {
			t.Fatal(err)
		}

		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		out := map[string]bool{}
		flatten("", m, out)

		return slices.Sorted(maps.Keys(out))
	}

	if diff := cmp.Diff(keys("en.json"), keys("pt-BR.json")); diff != "" {
		t.Fatalf("catalog keys differ (-en +pt-BR):\n%s", diff)
	}
}

func flatten(prefix string, m map[string]any, out map[string]bool) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		if sub, ok := v.(map[string]any); ok {
			flatten(key, sub, out)

			continue
		}

		out[key] = true
	}
}
