package render

import (
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

func RenderSidekickBig() {
	pterm.Println()

	s, _ := pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("Side", pterm.FgCyan.ToStyle()),
		putils.LettersFromStringWithStyle("kick", pterm.FgLightMagenta.ToStyle())).Srender()
	pterm.DefaultCenter.Println(s)

}
