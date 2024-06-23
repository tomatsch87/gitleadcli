package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-resty/resty/v2"
	"github.com/zalando/go-keyring"
)

type screen int

const (
	setupScreen screen = iota
	connectionScreen
)

type model struct {
	projectName  string
	gitlabHost   string
	accessToken  string
	currentField int
	currentScreen screen
	connectionStatus string
	err          error
}

func initialModel() model {
	return model{
		currentField: 0,
		currentScreen: setupScreen,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.currentScreen == setupScreen {
				if m.currentField == 2 {
					err := m.saveInputs()
					if err != nil {
						m.err = err
						return m, nil
					}
					return m, checkConnection
				}
				m.currentField = (m.currentField + 1) % 3
			} else if m.currentScreen == connectionScreen && m.connectionStatus != "Connection works!" {
				m = initialModel()
			}
		case "up":
			if m.currentField > 0 {
				m.currentField--
			}
		case "down":
			if m.currentField < 2 {
				m.currentField++
			}
		case "backspace":
			switch m.currentField {
			case 0:
				if len(m.projectName) > 0 {
					m.projectName = m.projectName[:len(m.projectName)-1]
				}
			case 1:
				if len(m.gitlabHost) > 0 {
					m.gitlabHost = m.gitlabHost[:len(m.gitlabHost)-1]
				}
			case 2:
				if len(m.accessToken) > 0 {
					m.accessToken = m.accessToken[:len(m.accessToken)-1]
				}
			}
		default:
			input := msg.String()
			input = strings.Trim(input, "[]")
			switch m.currentField {
			case 0:
				m.projectName += input
			case 1:
				m.gitlabHost += input
			case 2:
				m.accessToken += input
			}
		}
	case connectionStatusMsg:
		m.connectionStatus = string(msg)
		m.currentScreen = connectionScreen
	}
	return m, nil
}

func (m model) View() string {
	if m.currentScreen == setupScreen {
		return m.setupView()
	}
	return m.connectionView()
}

func (m model) setupView() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF00FF")).
		Bold(true).
		MarginBottom(1).
		PaddingLeft(1)

	inputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF00FF")).
		PaddingLeft(1).
		Width(40)

	activeInputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00FF00")).
		PaddingLeft(1).
		Width(40)

	s := titleStyle.Render("🌈 Welcome to GitLeadCLI 🌈\n")

	fields := []struct {
		prompt string
		value  string
	}{
		{"Project-Name:    ", m.projectName},
		{"GitLab-Host-URL: ", m.gitlabHost},
		{"Access-Token:    ", strings.Repeat("*", len(m.accessToken))},
	}

	for i, field := range fields {
		input := fmt.Sprintf("%-16s%s", field.prompt, field.value)
		if i == m.currentField {
			s += activeInputStyle.Render(input) + "\n"
		} else {
			s += inputStyle.Render(input) + "\n"
		}
	}

	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		MarginTop(1)
	s += instructionStyle.Render("(Press Enter to submit, Up/Down to switch fields)")

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			MarginTop(1)
		s += errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	return lipgloss.NewStyle().Margin(1, 0, 0, 1).Render(s)
}

func (m model) connectionView() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")).
		Bold(true).
		Margin(1, 0, 0, 1)

	if m.connectionStatus != "Connection works!" {
		style = style.Foreground(lipgloss.Color("#FF0000"))
	}

	s := style.Render(m.connectionStatus)

	if m.connectionStatus != "Connection works!" {
		s += "\n\n" + lipgloss.NewStyle().Render("Press Enter to return to setup")
	}

	return s
}

func (m *model) saveInputs() error {
	data, err := json.Marshal(map[string]string{
		"projectName": m.projectName,
		"gitlabHost":  m.gitlabHost,
		"accessToken": m.accessToken,
	})
	if err != nil {
		return err
	}
	return keyring.Set("GitLeadCLI", "config", string(data))
}

type connectionStatusMsg string

func checkConnection() tea.Msg {
	data, err := keyring.Get("GitLeadCLI", "config")
	if err != nil {
		return connectionStatusMsg("Error retrieving config: " + err.Error())
	}

	var config map[string]string
	err = json.Unmarshal([]byte(data), &config)
	if err != nil {
		return connectionStatusMsg("Error parsing config: " + err.Error())
	}

	client := resty.New()
	resp, err := client.R().
		SetHeader("PRIVATE-TOKEN", config["accessToken"]).
		Get(config["gitlabHost"] + "/api/v4/projects")

	if err != nil {
		return connectionStatusMsg("Connection failed: " + err.Error())
	}

	if resp.StatusCode() == 200 {
		return connectionStatusMsg("Connection works!")
	} else {
		return connectionStatusMsg(fmt.Sprintf("Connection failed: Status %d", resp.StatusCode()))
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}