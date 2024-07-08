package utils

import (
	"github.com/pterm/pterm"
)

var stagedSpinner = pterm.DefaultSpinner

func GetSpinner() pterm.SpinnerPrinter {
	stagedSpinner.Sequence = []string{"."}
	stagedSpinner.ShowTimer = false
	return stagedSpinner
}
