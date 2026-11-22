package internal

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
)

func FormatTime(seconds float64) string {
	total := int64(math.Floor(seconds))
	hours := total / 3600
	minutes := (total % 3600) / 60
	secs := total % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func InitSpinnerAndProgress() (spinner.Model, progress.Model) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#a54242"))

	p := progress.New(
		progress.WithoutPercentage(),
		progress.WithColors(lipgloss.Color("#cc6666"), lipgloss.Color("#cc6666")),
		progress.WithWidth(61),
	)
	return s, p
}

func DirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, nil
}
