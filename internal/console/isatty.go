package console

import (
	"os"

	"github.com/mattn/go-isatty"
)

// isinteractive reports whether stdin is connected to a real terminal
// when the backend is spawned as a child process by the electron app stdin
// is wired to a pipe (not a tty) so prompting for input would block forever
// code paths that need user input should check this and bail early when false
func IsInteractive() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
