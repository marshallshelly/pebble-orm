package output

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

func TestPaintFollowsTheOutput(t *testing.T) {
	saved := lipgloss.Writer.Profile
	t.Cleanup(func() { lipgloss.Writer.Profile = saved })

	lipgloss.Writer.Profile = colorprofile.NoTTY
	if got := StatusIcon("applied"); got != "✓" {
		t.Errorf("piped output must be plain text, got %q", got)
	}

	lipgloss.Writer.Profile = colorprofile.TrueColor
	if got := StatusIcon("applied"); !strings.Contains(got, "\x1b[") {
		t.Errorf("a colour terminal must get colour, got %q", got)
	}
}
