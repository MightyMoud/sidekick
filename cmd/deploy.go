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
package cmd

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type logMsg struct {
	LogLine string
}

type allDoneMsg struct {
	Duration time.Duration
	URL      string
}

type errorMsg struct {
	ErrorStr string
}
type setStateMsg struct {
	ActiveIndex int
}

type stage struct {
	Title    string
	Success  string
	Spinner  spinner.Model
	Logs     []string
	HasLogs  bool
	HasError bool
}

type deployModel struct {
	tea.Model
	ActiveIndex    int
	Stages         []stage
	Quitting       bool
	ViewportWidth  int
	ViewportHeight int
	AllDone        bool
	Duration       time.Duration
	URL            string
}

func sendLogsToTUI(source io.ReadCloser, p *tea.Program) {
	reader := bufio.NewReader(source)
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
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a new version of your application to your VPS using Sidekick",
	Long: `This command deploys a new version of your application to your VPS. 
It assumes that your VPS is already configured and that your application is ready for deployment`,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		if configErr := utils.ViperInit(); configErr != nil {
			pterm.Error.Println("Sidekick config not found - Run sidekick init")
			os.Exit(1)
		}
		if !utils.FileExists("./sidekick.yml") {
			pterm.Error.Println(`Sidekick config not found in current directory Run sidekick launch`)
			os.Exit(1)
		}

		cmdStages := []stage{
			makeStage("Validating connection with VPS", "VPS is reachable", false),
			makeStage("Building latest docker image of your app", "Latest docker image built", true),
			makeStage("Saving docker image locally", "Image saved successfully", false),
			makeStage("Moving image to your server", "Image moved and loaded successfully", false),
			makeStage("Deploying a new version of your application", "🙌 Deployed new version successfully 🙌", false),
		}
		p := tea.NewProgram(newModel(cmdStages))

		appConfig, loadError := utils.LoadAppConfig()
		if loadError != nil {
			panic(loadError)
		}
		replacer := strings.NewReplacer(
			"$service_name", appConfig.Name,
			"$app_port", fmt.Sprint(appConfig.Port),
		)

		go func() {
			sshClient, err := utils.Login(viper.Get("serverAddress").(string), "sidekick")
			if err != nil {
				// loginSpinner.Fail("Something went wrong logging in to your VPS")
				panic(err)
			}
			p.Send(setStateMsg{ActiveIndex: 1})

			envFileChanged := false
			currentEnvFileHash := ""
			if appConfig.Env.File != "" {
				envFileContent, envFileErr := os.ReadFile(fmt.Sprintf("./%s", appConfig.Env.File))
				if envFileErr != nil {
					pterm.Error.Println("Unable to process your env file")
					os.Exit(1)
				}
				currentEnvFileHash = fmt.Sprintf("%x", md5.Sum(envFileContent))
				envFileChanged = appConfig.Env.Hash != currentEnvFileHash
				if envFileChanged {
					// encrypt new env file
					envCmd := exec.Command("sh", "-s", "-", viper.Get("publicKey").(string), fmt.Sprintf("./%s", appConfig.Env.File))
					envCmd.Stdin = strings.NewReader(utils.EnvEncryptionScript)
					if envCmdErr := envCmd.Run(); envCmdErr != nil {
						pterm.Error.Printfln("Something went wrong handling your env file: %s", envCmdErr)
						os.Exit(1)
						panic(envCmdErr)
					}
					encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appConfig.Name)))
					encryptSync.Run()
				}
			}
			defer os.Remove("encrypted.env")

			cwd, _ := os.Getwd()

			imgFileName := fmt.Sprintf("%s-latest.tar", appConfig.Name)
			dockerBuildCmd := exec.Command("docker", "build", "--tag", appConfig.Name, "--progress=plain", "--platform=linux/amd64", cwd)
			dockerBuildCmdErrPipe, _ := dockerBuildCmd.StderrPipe()
			go sendLogsToTUI(dockerBuildCmdErrPipe, p)

			if dockerBuildErr := dockerBuildCmd.Run(); dockerBuildErr != nil {
				p.Send(errorMsg{})
			}

			time.Sleep(time.Millisecond * 1000)

			p.Send(setStateMsg{ActiveIndex: 2})

			imgSaveCmd := exec.Command("docker", "save", "-o", imgFileName, appConfig.Name)
			errChan := make(chan string)
			imgSaveCmdErrPipe, _ := imgSaveCmd.StderrPipe()

			go func() {
				reader := bufio.NewReader(imgSaveCmdErrPipe)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							break
						}
					}
					if line != "\n" {
						p.Send(logMsg{LogLine: line})
						errChan <- line
					}
				}
			}()

			if imgSaveCmdErr := imgSaveCmd.Run(); imgSaveCmdErr != nil {
				p.Send(errorMsg{})
			}

			time.Sleep(time.Millisecond * 500)

			p.Send(setStateMsg{ActiveIndex: 3})

			remoteDist := fmt.Sprintf("%s@%s:./%s", "sidekick", viper.GetString("serverAddress"), appConfig.Name)
			imgMoveCmd := exec.Command("scp", "-C", "-v", imgFileName, remoteDist)
			imgMoveCmdErrorPipe, _ := imgMoveCmd.StderrPipe()

			go func() {
				scanner := bufio.NewScanner(imgMoveCmdErrorPipe)
				for scanner.Scan() {
					p.Send(logMsg{LogLine: scanner.Text() + "\n"})
					time.Sleep(time.Millisecond * 100)
				}
			}()

			if imgMovCmdErr := imgMoveCmd.Run(); imgMovCmdErr != nil {
				p.Send(errorMsg{})
			}
			// os.Remove(imgFileName)

			time.Sleep(time.Millisecond * 500)
			p.Send(setStateMsg{ActiveIndex: 4})

			dockerLoadOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s-latest.tar", appConfig.Name, appConfig.Name))
			if sessionErr != nil {
				log.Fatal("Issue happened loading docker image")
			}

			go func() {
				p.Send(logMsg{LogLine: <-dockerLoadOutChan + "\n"})
				time.Sleep(time.Millisecond * 100)
			}()

			if appConfig.Env.File != "" {
				deployScript := replacer.Replace(utils.DeployAppWithEnvScript)
				_, runVersionOutChan, sessionErr := utils.RunCommand(sshClient, deployScript)
				if sessionErr != nil {
					panic(sessionErr)
				}
				go func() {
					p.Send(logMsg{LogLine: <-runVersionOutChan + "\n"})
					time.Sleep(time.Millisecond * 100)
				}()
			} else {
				deployScript := replacer.Replace(utils.DeployAppScript)
				_, runOutChan, sessionErr := utils.RunCommand(sshClient, deployScript)
				if sessionErr != nil {
					panic(sessionErr)
				}
				go func() {
					p.Send(logMsg{LogLine: <-runOutChan + "\n"})
				}()
				time.Sleep(time.Second * 2)
			}

			cleanOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && rm %s", appConfig.Name, fmt.Sprintf("%s-latest.tar", appConfig.Name)))
			if sessionErr != nil {
				log.Fatal("Issue happened cleaning up the image file")
			}
			go func() {
				p.Send(logMsg{LogLine: <-cleanOutChan + "\n"})
				time.Sleep(time.Millisecond * 100)
			}()

			latestVersion := strings.Split(appConfig.Version, "")[1]
			latestVersionInt, _ := strconv.ParseInt(latestVersion, 0, 64)
			appConfig.Version = fmt.Sprintf("V%d", latestVersionInt+1)
			// env file changed ? -> update hash
			if envFileChanged {
				appConfig.Env.Hash = currentEnvFileHash
			}
			ymlData, _ := yaml.Marshal(&appConfig)
			os.WriteFile("./sidekick.yml", ymlData, 0644)

			time.Sleep(time.Millisecond * 500)
			p.Send(allDoneMsg{Duration: time.Since(start), URL: appConfig.Url})
		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).MarginRight(1).MarginLeft(1)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).MarginLeft(1)
	cancelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Faint(true).MarginLeft(1)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).MarginLeft(1)
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginLeft(1)
	allDoneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).MarginTop(1).MarginLeft(1).MarginBottom(1)
	appStyle     = lipgloss.NewStyle()
)

func getLogContainerStyle(m deployModel) lipgloss.Style {
	return lipgloss.
		NewStyle().
		Width(int(0.8 * float64(m.ViewportWidth))).
		Height(0).
		MarginLeft(1).
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
		AllDone:     false,
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
		logStage.Logs = append(logStage.Logs, msg.LogLine)
		m.Stages[m.ActiveIndex] = logStage

		return m, nil

	case errorMsg:
		logStage := m.Stages[m.ActiveIndex]
		logStage.HasError = true
		if msg.ErrorStr != "" {
			logStage.Logs = append(logStage.Logs, msg.ErrorStr)
		}
		m.Stages[m.ActiveIndex] = logStage

		return m, tea.Quit

	case setStateMsg:
		m.ActiveIndex = msg.ActiveIndex

		return m, m.Stages[m.ActiveIndex].Spinner.Tick

	case allDoneMsg:
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

func (m deployModel) View() string {
	var s string
	printSlice := []string{}

	printSlice = append(printSlice, getBannerStyle(m).Render("Deploying a new version of your app 😏"))

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
					u := tree.Root("⚠ " + stage.Title).Child(stage.Logs[:1])
					printSlice = append(printSlice, errorStyle.Render(u.String()))
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

func makeStage(title string, success string, hasLogs bool) stage {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.MiniDot

	logs := []string{}

	return stage{
		Spinner: s,
		Title:   title,
		Success: success,
		Logs:    logs,
		HasLogs: hasLogs,
	}
}
