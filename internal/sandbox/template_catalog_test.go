package sandbox

import "testing"

func TestIsStandardTemplateRecognizesProviderScopedName(t *testing.T) {
	for _, name := range []string{"yuheng", "team/yuheng", "project-b89e/Yuheng"} {
		if !isStandardTemplate(name) {
			t.Fatalf("expected %q to identify the Yuheng standard template", name)
		}
	}
	if isStandardTemplate("yuheng-custom") {
		t.Fatal("custom template must not be treated as the standard template")
	}
}
