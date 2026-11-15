package desktop

import (
	"errors"
	"net/url"
	"os/exec"
	"runtime"
)

// Only the server-generated, fixed Frontier PKCE URL may open the system browser.
func OpenAuthorizationURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "auth.frontierstore.net" || u.Path != "/auth" || u.User != nil || u.Fragment != "" {
		return errors.New("unsupported authorization URL")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", raw)
	} else {
		cmd = exec.Command("xdg-open", raw)
	}
	hideAuthorizationWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
