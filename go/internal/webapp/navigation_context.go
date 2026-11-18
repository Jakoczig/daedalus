package webapp

import (
	"context"
	"github.com/Jakoczig/eridion/go/internal/community"
	"github.com/Jakoczig/eridion/go/internal/navcontext"
	"strings"
)

func strictDecode(raw []byte, v any) error { return navcontext.Decode(raw, v) }
func (s *Server) contextView(p socketPrincipal) any {
	return map[string]any{"navigation": s.game.NavigationContext(), "policy": s.displayPolicy(p)}
}
func (s *Server) systemView(ctx context.Context, name string, lookup bool) any {
	p, local := s.game.KnownSystem(name)
	var external community.View
	if lookup {
		if p.Address != "" {
			external = s.community.Refresh(ctx, p.Name, p.Address)
		} else {
			external = s.community.Lookup(ctx, name)
		}
	} else {
		if p.Address != "" {
			external = s.community.View(p.Name, p.Address)
		} else {
			external = s.community.Named(name)
		}
	}
	if p.Name == "" && external.SystemAddress != "" {
		p = navcontext.Point{Name: external.SystemName, Address: external.SystemAddress, Coordinates: external.Coordinates, Source: "EDSM"}
	}
	availability := "unavailable"
	if local.SystemName != "" && len(local.Bodies) > 0 {
		availability = "local_cached"
	} else if len(external.Bodies) > 0 {
		availability = "community_cached"
	}
	return map[string]any{"point": p, "local": local, "community": external, "availability": availability}
}
func cleanSystemName(name string) bool {
	return name != "" && len(name) <= 128 && !strings.ContainsAny(name, "\r\n\x00")
}
func (s *Server) LauncherReady() <-chan struct{} { return s.launcherReady }
