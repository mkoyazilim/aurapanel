package main

import (
	"strings"
	"testing"
)

func TestReplacePanelEdgeExtProcessorBlockUpdatesAddress(t *testing.T) {
	input := strings.Join([]string{
		"docRoot                   /usr/local/lsws/Example/html/",
		"",
		panelEdgeExtprocBeginMarker,
		"extprocessor aurapanel_gateway {",
		"  type                    proxy",
		"  address                 127.0.0.1:8090",
		"  maxConns                1000",
		"}",
		"",
		"websocket /api/v1/terminal/ws {",
		"  address                 127.0.0.1:8090",
		"}",
		panelEdgeExtprocEndMarker,
		"",
		"index  {",
		"  useServer               0",
		"}",
	}, "\n")

	out, err := replacePanelEdgeExtProcessorBlock(input, "127.0.0.1:9443")
	if err != nil {
		t.Fatalf("replacePanelEdgeExtProcessorBlock returned error: %v", err)
	}
	if !strings.Contains(out, "address                 127.0.0.1:9443") {
		t.Fatalf("updated upstream address missing in output:\n%s", out)
	}
	if strings.Contains(out, "address                 127.0.0.1:8090") {
		t.Fatalf("old upstream address still present in output:\n%s", out)
	}
	if !strings.Contains(out, "websocket /api/v1/terminal/ws {") {
		t.Fatalf("websocket proxy block missing in output:\n%s", out)
	}
	if !strings.Contains(out, panelEdgeExtprocBeginMarker) || !strings.Contains(out, panelEdgeExtprocEndMarker) {
		t.Fatalf("managed markers should be preserved")
	}
}

func TestReplacePanelEdgeExtProcessorBlockRequiresMarkers(t *testing.T) {
	_, err := replacePanelEdgeExtProcessorBlock("docRoot /var/www/html\n", "127.0.0.1:9443")
	if err == nil {
		t.Fatalf("expected missing marker error")
	}
}
