package main

import (
	"strings"
	"testing"
)

func TestRenderOLSManagedListenerMapBlockKeepsExampleFallback(t *testing.T) {
	block := renderOLSManagedListenerMapBlock([]olsManagedSite{
		{
			Site: Website{Domain: "aurapanel.info"},
			Aliases: []string{
				"aurapanel.info",
				"www.aurapanel.info",
			},
		},
	})

	if !strings.Contains(block, "map                      AuraPanel_aurapanel_info aurapanel.info, www.aurapanel.info") {
		t.Fatalf("managed site mapping missing from listener block: %s", block)
	}
	if !strings.Contains(block, "map                      Example *") {
		t.Fatalf("example fallback mapping missing from listener block: %s", block)
	}
}

func TestSiteSystemOwnerSanitizesWebsiteOwner(t *testing.T) {
	owner := siteSystemOwner(Website{Owner: " Demo Owner "})
	if owner != "demo_owner" {
		t.Fatalf("expected sanitized system owner, got %q", owner)
	}
}
