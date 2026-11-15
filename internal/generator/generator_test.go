package generator

import (
	"strings"
	"testing"
)

func TestStandaloneHTMLTransparencyAttributes(t *testing.T) {
	html := BuildStandaloneHTMLWithBackground("Test Title", 800, 600, "<path d='M0 0'/>", true, "#123456")
	if !strings.Contains(html, `data-bg-transparent="true"`) {
		t.Errorf("expected html to contain data-bg-transparent=\"true\"")
	}
	if !strings.Contains(html, `data-bg-color="#123456"`) {
		t.Errorf("expected html to contain data-bg-color=\"#123456\"")
	}

	htmlOpaque := BuildStandaloneHTMLWithBackground("Test Title", 800, 600, "<path d='M0 0'/>", false, "#abcdef")
	if !strings.Contains(htmlOpaque, `data-bg-transparent="false"`) {
		t.Errorf("expected html to contain data-bg-transparent=\"false\"")
	}
	if !strings.Contains(htmlOpaque, `data-bg-color="#abcdef"`) {
		t.Errorf("expected html to contain data-bg-color=\"#abcdef\"")
	}
}

func TestBuildStandaloneWidgetHTML(t *testing.T) {
	svg := `<svg width="400" height="300"><defs><filter id="f1"/></defs><rect filter="url(#f1)"/></svg>`
	out := BuildStandaloneWidgetHTML("Test Widget", 400, 300, svg, "inst1")
	if !strings.Contains(out, `id="container-inst1"`) {
		t.Errorf("expected container-inst1 in output")
	}
	if !strings.Contains(out, `id="inst1__f1"`) {
		t.Errorf("expected inst1__f1 mapped ID in output")
	}
	if !strings.Contains(out, `filter="url(#inst1__f1)"`) {
		t.Errorf("expected filter url to be remapped in output")
	}
}
