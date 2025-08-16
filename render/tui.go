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
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
)

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).MarginRight(1).MarginLeft(1)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).MarginLeft(1)
	cancelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Faint(true).MarginLeft(1)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).MarginLeft(1)
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginLeft(1)
	allDoneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).MarginTop(1).MarginLeft(1).MarginBottom(1)
	appStyle     = lipgloss.NewStyle()
)

func (m TuiModel) Init() tea.Cmd {
	return m.Stages[m.ActiveIndex].Spinner.Tick
}

func (m TuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		m.Quitting = true

		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.ViewportHeight = msg.Height
		m.ViewportWidth = msg.Width
		return m, nil

	case LogMsg:
		logStage := m.Stages[m.ActiveIndex]
		logStage.Logs = append(logStage.Logs, msg.LogLine)
		m.Stages[m.ActiveIndex] = logStage

		return m, nil

	case ErrorMsg:
		logStage := m.Stages[m.ActiveIndex]
		logStage.HasError = true
		if msg.ErrorStr != "" {
			logStage.Logs = append(logStage.Logs, msg.ErrorStr)
		}
		m.Stages[m.ActiveIndex] = logStage

		WriteStageLogs(logStage, m.ActiveIndex)

		return m, tea.Quit

	case NextStageMsg:
		m.ActiveIndex = m.ActiveIndex + 1

		return m, m.Stages[m.ActiveIndex].Spinner.Tick

	case AllDoneMsg:
		m.AllDone = true
		m.URL = msg.URL
		m.Duration = msg.Duration

		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Stages[m.ActiveIndex].Spinner, cmd = m.Stages[m.ActiveIndex].Spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m TuiModel) View() string {
	var s string
	printSlice := []string{}

	printSlice = append(printSlice, getBannerStyle(m).Render(m.BannerMsg))

	var logs string
	for _, res := range m.Stages[m.ActiveIndex].Logs {
		logs += res
	}

	for index, stage := range m.Stages {
		if !m.AllDone {
			if index < m.ActiveIndex {
				printSlice = append(printSlice, successStyle.Render("✔ "+stage.Success))
			} else if index == m.ActiveIndex {
				if !stage.HasError {
					printSlice = append(printSlice, stage.Spinner.View()+stage.Title)
				} else {
					u := tree.Root("⚠ " + stage.Title).Child(stage.Logs)
					printSlice = append(printSlice, errorStyle.Render(u.String()))
					printSlice = append(printSlice, allDoneStyle.Render("⚠️ Check sidekick.logs.txt for more details"))
				}
				if stage.HasLogs && !stage.HasError {
					var t string
					if !stage.HasError {
						l := len(stage.Logs)
						if l < 5 {
							t = getLogContainerStyle(m).Render(stage.Logs...)
						} else {
							t = getLogContainerStyle(m).Render(stage.Logs[l-5:]...)
						}
					} else {
						t = getLogContainerStyle(m).Render(stage.Logs...)
					}
					printSlice = append(printSlice, t)
				}
			} else if index > m.ActiveIndex {
				var text string
				if m.Quitting {
					text = cancelStyle.Render("CANCELLED " + stage.Title)
				} else {
					text = pendingStyle.Render("󰚭 " + stage.Title)
				}
				printSlice = append(printSlice, pendingStyle.Render(text))
			}
		} else {
			printSlice = append(printSlice, successStyle.Render("✔ "+stage.Success))
		}
	}

	if m.AllDone {
		printSlice = append(printSlice, allDoneStyle.Render("🚀 Deployed successfully in "+m.Duration.String()+".\n"+"😎 View your app at https://"+m.URL))
	}

	s += lipgloss.JoinVertical(lipgloss.Top, printSlice...)

	s += "\n"

	if m.Quitting {
		s += "\n"
	}

	return appStyle.Render(s)
}

func getLogContainerStyle(m TuiModel) lipgloss.Style {
	return lipgloss.
		NewStyle().
		Width(int(0.8 * float64(m.ViewportWidth))).
		Height(0).
		MarginLeft(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Foreground(lipgloss.Color("white")).Faint(true)
}
func getBannerStyle(m TuiModel) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("white")).
		Background(lipgloss.Color("#414868")).
		Width(m.ViewportWidth).
		Padding(1).
		Align(lipgloss.Center).
		MarginBottom(1).
		MarginTop(1)
}

func MakeStage(title string, success string, hasLogs bool) Stage {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.MiniDot

	logs := []string{}

	return Stage{
		Spinner: s,
		Title:   title,
		Success: success,
		Logs:    logs,
		HasLogs: hasLogs,
	}
}

func SendLogsToTUI(source io.ReadCloser, p *tea.Program) {
	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "\n" {
			p.Send(LogMsg{LogLine: scanner.Text() + "\n"})
			time.Sleep(time.Millisecond * 50)
		}
	}
}

// WriteStageLogs writes the logs from a specific stage to sidekick.logs.txt
func WriteStageLogs(stage Stage, stageIndex int) error {
	if len(stage.Logs) == 0 {
		return nil
	}

	file, err := os.OpenFile("sidekick.logs.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("\n=== STAGE %d ERROR LOG - %s ===\n", stageIndex+1, timestamp)
	header += fmt.Sprintf("Stage: %s\n", stage.Title)
	header += "=== LOGS START ===\n"

	if _, err := file.WriteString(header); err != nil {
		return err
	}

	for _, log := range stage.Logs {
		if _, err := file.WriteString(log); err != nil {
			return err
		}
		if !strings.HasSuffix(log, "\n") {
			if _, err := file.WriteString("\n"); err != nil {
				return err
			}
		}
	}

	footer := "=== LOGS END ===\n\n"
	if _, err := file.WriteString(footer); err != nil {
		return err
	}

	return nil
}
