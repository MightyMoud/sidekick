/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

Licensed under the GNU AGPL License, Version 3.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.gnu.org/licenses/agpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
