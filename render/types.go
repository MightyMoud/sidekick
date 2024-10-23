/*
Copyright © 2024 Mahmoud Mousa <m.mousa@hey.com>

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
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type LogMsg struct {
	LogLine string
}

type AllDoneMsg struct {
	Duration time.Duration
	URL      string
}

type ErrorMsg struct {
	ErrorStr string
}
type NextStageMsg struct{}

type Stage struct {
	Title    string
	Success  string
	Spinner  spinner.Model
	Logs     []string
	HasLogs  bool
	HasError bool
}

type TuiModel struct {
	tea.Model
	ActiveIndex    int
	Stages         []Stage
	Quitting       bool
	ViewportWidth  int
	ViewportHeight int
	AllDone        bool
	Duration       time.Duration
	URL            string
	BannerMsg      string
}
