/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

Licensed under the GNU GPL License, Version 3.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.gnu.org/licenses/gpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package render

import (
	"os"

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

func RenderKeyValidation(resultLines []string, keyHash string, hostname string) {
	startColor := pterm.NewRGB(0, 255, 255)
	endColor := pterm.NewRGB(255, 0, 255)

	pterm.DefaultCenter.Print(keyHash)
	for i := 0; i < len(resultLines[1:]); i++ {
		fadeFactor := float32(i) / float32(20)
		currentColor := startColor.Fade(0, 1, fadeFactor, endColor)
		pterm.DefaultCenter.Print(currentColor.Sprint(resultLines[1:][i]))
	}
	prompt := pterm.DefaultInteractiveContinue

	pterm.DefaultCenter.Printf(pterm.FgYellow.Sprintf("This is the ASCII art and fingerprint of your VPS's public key at %s", hostname))
	pterm.DefaultCenter.Printf(pterm.FgYellow.Sprint("Please confirm you want to continue with the connection"))
	pterm.DefaultCenter.Printf(pterm.FgYellow.Sprint("Sidekick will add this host/key pair to known_hosts"))
	pterm.Println()

	prompt.DefaultText = "Would you like to proceed?"
	prompt.Options = []string{"yes", "no"}
	if result, _ := prompt.Show(); result != "yes" {
		pterm.Error.Println("In order to continue, you need to accept this.")
		os.Exit(0)
	}
}
