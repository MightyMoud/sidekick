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

func GetSpinners(multi pterm.MultiPrinter, titles []string) []*pterm.SpinnerPrinter {
	spinners := make([]*pterm.SpinnerPrinter, len(titles))
	for i, title := range titles {
		newSpinner := GetSpinner().WithWriter(multi.NewWriter()).WithText(title)
		spinners[i] = newSpinner
		newSpinner.Start()
	}
	return spinners
}
