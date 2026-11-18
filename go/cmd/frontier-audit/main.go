// Command frontier-audit is an explicit local owner diagnostic, never a server.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/providers"
	"os"
)

func main() {
	profile := flag.String("profile", "", "Existing native profile, opened read-only")
	output := flag.String("output", "", "Private diagnostic directory outside the profile")
	kind := flag.String("read", "", "One endpoint: profile, market, shipyard, fleetcarrier, communitygoals")
	ack := flag.Bool("owner-read-only", false, "Explicitly authorize one live read using this OS owner's existing consent")
	flag.Parse()
	if !*ack || *profile == "" || *output == "" || *kind == "" {
		fmt.Fprintln(os.Stderr, "Explicit --owner-read-only --profile --output --read required; no request made")
		os.Exit(2)
	}
	if e := providers.AuditNativeFrontier(context.Background(), *profile, *output, *kind); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("Sanitized structural diagnostic saved; no raw response or credential written")
}
