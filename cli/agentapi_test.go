package cli

import "testing"

func TestModelSpecs(t *testing.T) {
	for _, spec := range []string{"qwen2.5:7b", "gpt-x,vision,context=200000,output=32000,effort=high", "m,effort=low"} {
		m, err := parseModelSpec(spec)
		if err != nil {
			t.Fatal(err)
		}
		if got := modelSpec(m); got != spec {
			t.Errorf("%s: written back as %s", spec, got)
		}
	}
	for _, bad := range []string{",vision", "m,context=lots", "m,fast"} {
		if _, err := parseModelSpec(bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
}
