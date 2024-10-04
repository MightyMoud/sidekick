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
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/cobra"
)

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
		s := spinner.New()
		s.Style = spinnerStyle
		s.Spinner = spinner.Jump

		logs := make([]string, 5)
		logs = append(logs, "")

		logs2 := make([]string, 5)
		logs2 = append(logs2, "")

		cmdStages := map[string]stage{
			"login": {
				Spinner: s,
				Title:   "Logining in yo",
				Logs:    logs,
			},
			"second": {
				Spinner: s,
				Title:   "Second stage man",
				Logs:    logs2,
			},
		}
		p := tea.NewProgram(newModel(cmdStages))
		cwd, _ := os.Getwd()
		// var stdErrBuff bytes.Buffer
		dockerBuildCommd := exec.Command("sh", "-s", "-", "app", cwd, "latest")
		dockerBuildCommd.Stdin = strings.NewReader(utils.DockerBuildAndSaveScript)
		secCommand := exec.Command("pwd")
		// dockerBuildCommd.Stderr = &stdErrBuff
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
				p.Send(LogMsg{Stage: "login", LogLine: line})
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
				p.Send(LogMsg{Stage: "second", LogLine: line})
			}
		}()

		go func() {
			if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
				// pterm.Error.Printfln("Failed to build docker image with the following error: \n%s", stdErrBuff.String())
				// os.Exit(1)
			}
			time.Sleep(time.Millisecond * 2000)
			p.Send(SetStateMsg{ActiveStage: "second"})
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

var (
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("150"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	dotStyle      = helpStyle.UnsetMargins()
	durationStyle = dotStyle
	appStyle      = lipgloss.NewStyle().Margin(1, 2, 0, 2)
)

func getLogContainerStyle(m model) lipgloss.Style {
	return lipgloss.
		NewStyle().
		Width(int(0.8 * float64(m.ViewportWidth))).
		Height(0).
		PaddingLeft(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69"))

}

type LogMsg struct {
	Stage   string
	LogLine string
}
type SetStateMsg struct {
	ActiveStage string
}

type stage struct {
	Title   string
	Spinner spinner.Model
	Logs    []string
}

type model struct {
	ActiveStage    string
	Stages         map[string]stage
	Quitting       bool
	ViewportWidth  int
	ViewportHeight int
}

func newModel(cmdStages map[string]stage) model {
	const logsDepth = 5
	s := spinner.New()
	s.Spinner = spinner.Jump
	s.Style = spinnerStyle

	return model{
		Stages:      cmdStages,
		ActiveStage: "login",
		Quitting:    false,
	}
}

func (m model) Init() tea.Cmd {
	return m.Stages[m.ActiveStage].Spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.Quitting = true
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.ViewportHeight = msg.Height
		m.ViewportWidth = msg.Width
		return m, nil

	case LogMsg:
		logStage := m.Stages[msg.Stage]
		logStage.Logs = append(logStage.Logs[1:], msg.LogLine)
		m.Stages[msg.Stage] = logStage

		return m, nil

	case SetStateMsg:
		m.ActiveStage = msg.ActiveStage

		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		stage := m.Stages[m.ActiveStage]
		spinnerTarget := stage.Spinner
		spinnerTarget, cmd = spinnerTarget.Update(msg)
		stage.Spinner = spinnerTarget
		m.Stages[m.ActiveStage] = stage
		return m, cmd
	default:
		return m, nil
	}
}

func (m model) View() string {
	var s string

	if m.Quitting {
		s += "That’s all for today!"
	}

	var logs string
	for _, res := range m.Stages[m.ActiveStage].Logs {
		logs += res
	}

	printSlice := []string{}
	for name, stage := range m.Stages {
		printSlice = append(printSlice, stage.Spinner.View()+stage.Title)
		if name == m.ActiveStage {
			printSlice = append(printSlice, getLogContainerStyle(m).Render(m.Stages[m.ActiveStage].Logs...))
		}
	}

	s += lipgloss.JoinVertical(lipgloss.Top, printSlice...)

	if !m.Quitting {
		s += helpStyle.Render("Press any key to exit")
	}

	if m.Quitting {
		s += "\n"
	}

	return appStyle.Render(s)
}

func init() {
	rootCmd.AddCommand(teaCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// teaCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// teaCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
