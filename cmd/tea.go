/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type logMsg struct {
	LogLine string
}
type setStateMsg struct {
	ActiveIndex int
}

type stage struct {
	Title   string
	Success string
	Spinner spinner.Model
	Logs    []string
}

type deployModel struct {
	tea.Model
	ActiveIndex    int
	Stages         []stage
	Quitting       bool
	ViewportWidth  int
	ViewportHeight int
}

// teaCmd represents the tea command
var teaCmd = &cobra.Command{
	Use:   "tea",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		cmdStages := []stage{
			makeStage("Logging in to your server", "Logged in successfully"),
			makeStage("Doing something", "We did it"),
		}

		p := tea.NewProgram(newModel(cmdStages))
		// cwd, _ := os.Getwd()
		// f, err := os.Create("my_stderr.log")
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// defer f.Close()
		// var stdErrBuff bytes.Buffer
		dockerBuildCommd := exec.Command("docker", "build", "--tag", "app", ".")
		// dockerBuildCommd.Stdin = strings.NewReader(utils.DockerBuildAndSaveScript)
		secCommand := exec.Command("pwd")

		// p, _ := dockerBuildCommd.StdoutPipe()
		e, _ := dockerBuildCommd.StderrPipe()
		y, _ := secCommand.StdoutPipe()

		go func() {
			reader := bufio.NewReader(e)
			for {
				line, err := reader.ReadString('\n')

				if err != nil {
					if err == io.EOF {
						break
					}
				}
				if line != "\n" {
					p.Send(logMsg{LogLine: line})
				}
			}
		}()

		go func() {
			reader := bufio.NewReader(y)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						break
					}
				}
				p.Send(logMsg{LogLine: line})
			}
		}()

		go func() {
			if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
				// pterm.Error.Printfln("Failed to build docker image with the following error: \n%s", stdErrBuff.String())
				// os.Exit(1)
			}
			time.Sleep(time.Millisecond * 2000)
			p.Send(setStateMsg{ActiveIndex: 1})
			time.Sleep(time.Millisecond * 2000)
			if secCommandErr := secCommand.Run(); secCommandErr != nil {
				// pterm.Error.Printfln("Failed to build docker image with the following error: \n%s", stdErrBuff.String())
				// os.Exit(1)
			}

		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}

	},
}

func init() {
	rootCmd.AddCommand(teaCmd)

}

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).MarginRight(1)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	cancelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Faint(true)
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	appStyle     = lipgloss.NewStyle()
)

func getLogContainerStyle(m deployModel) lipgloss.Style {
	return lipgloss.
		NewStyle().
		Width(int(0.8 * float64(m.ViewportWidth))).
		Height(0).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Foreground(lipgloss.Color("white")).Faint(true)
}
func getBannerStyle(m deployModel) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("white")).
		Background(lipgloss.Color("#414868")).
		Width(m.ViewportWidth).
		Padding(1).
		Align(lipgloss.Center).
		MarginBottom(1).
		MarginTop(1)
}

func newModel(cmdStages []stage) deployModel {
	return deployModel{
		Stages:      cmdStages,
		ActiveIndex: 0,
		Quitting:    false,
	}
}

func (m deployModel) Init() tea.Cmd {
	return m.Stages[m.ActiveIndex].Spinner.Tick
}

func (m deployModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.Quitting = true
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.ViewportHeight = msg.Height
		m.ViewportWidth = msg.Width
		return m, nil

	case logMsg:
		logStage := m.Stages[m.ActiveIndex]
		logStage.Logs = append(logStage.Logs[1:], msg.LogLine)
		m.Stages[m.ActiveIndex] = logStage

		return m, nil

	case setStateMsg:
		m.ActiveIndex = msg.ActiveIndex

		return m, m.Stages[m.ActiveIndex].Spinner.Tick

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Stages[m.ActiveIndex].Spinner, cmd = m.Stages[m.ActiveIndex].Spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m deployModel) View() string {
	var s string
	printSlice := []string{}

	printSlice = append(printSlice, getBannerStyle(m).Render("Deploying a new version of your app 😏"))

	var logs string
	for _, res := range m.Stages[m.ActiveIndex].Logs {
		logs += res
	}

	for index, stage := range m.Stages {
		if index < m.ActiveIndex {
			printSlice = append(printSlice, successStyle.Render("✔ "+stage.Success))
		} else if index == m.ActiveIndex {
			printSlice = append(printSlice, stage.Spinner.View()+stage.Title)
			printSlice = append(printSlice, getLogContainerStyle(m).Render(stage.Logs...))
		} else if index > m.ActiveIndex {
			var text string
			if m.Quitting {
				text = cancelStyle.Render("CANCELLED " + stage.Title)
			} else {
				text = pendingStyle.Render("󰚭 " + stage.Title)
			}
			printSlice = append(printSlice, pendingStyle.Render(text))
		}
	}

	s += lipgloss.JoinVertical(lipgloss.Top, printSlice...)

	s += "\n"

	if m.Quitting {
		s += "\n"
	}

	return appStyle.Render(s)
}

func makeStage(title string, success string) stage {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.MiniDot

	logs := make([]string, 5)
	logs = append(logs, "")

	return stage{
		Spinner: s,
		Title:   title,
		Success: success,
		Logs:    logs,
	}
}
