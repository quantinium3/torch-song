package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/BurntSushi/toml"
	"github.com/haarshmap/torch-song/internal"
	"github.com/ioksc/gompv"
)

type model struct {
	mpv          *gompv.Client
	title        string
	timePos      string
	timePosFlag  float64
	paused       string
	repeat       bool
	repeatFlag   string
	playlistPos  int
	duration     string
	durationFlag float64
	spinner      spinner.Model
	progress     progress.Model
	data         map[string]any
}

var (
	container = lipgloss.NewStyle().
		Bold(true).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#cc6666")).
		BorderBackground(lipgloss.Color("#282a2e")).
		Padding(1, 2)
)

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keybinds := m.data["Keybinds"].(map[string]any)
	switch msg := msg.(type) {
	case gompv.Event:
		if name, value, ok := msg.PropertyString(); ok && name == "media-title" {
			m.title = value
		}
		if name, value, ok := msg.PropertyFloat(); ok && name == "time-pos" {
			m.timePos = internal.FormatTime(value)
			m.timePosFlag = value
		}
		if name, value, ok := msg.PropertyFloat(); ok && name == "duration" {
			m.duration = internal.FormatTime(value)
			m.durationFlag = value
		}
		if name, value, ok := msg.PropertyFloat(); ok && name == "playlist-pos" {
			index := value
			m.playlistPos = int(index)
		}
		if name, value, ok := msg.PropertyBool(); ok && name == "pause" {
			if value {
				m.paused = "󰏤"
			}
			if !value {
				m.paused = ""
			}
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case keybinds["play"]:
			m.mpv.TogglePause()
			return m, nil
		case keybinds["next"]:
			m.mpv.Next()
			return m, nil
		case keybinds["prev"]:
			m.mpv.Prev()
			return m, nil
		case keybinds["mute"]:
			m.mpv.ToggleMute()
			return m, nil
		case keybinds["shuffle"]:
			m.mpv.Shuffle()
			return m, nil
		case keybinds["vol-up"]:
			total, _ := m.mpv.GetVolume()
			total += 5
			if total > 100 {
				total = 100
			}
			m.mpv.SetVolume(total)
			return m, nil
		case keybinds["vol-down"]:
			total, _ := m.mpv.GetVolume()
			total -= 5
			if total < 0 {
				total = 0
			}
			m.mpv.SetVolume(total)
			return m, nil
		case keybinds["repeat"]:
			m.repeat = !m.repeat
			if m.repeat {
				m.repeatFlag = "repeat"
				m.mpv.Send("set_property", "loop-file", "inf")
			} else {
				m.repeatFlag = ""
				m.mpv.Send("set_property", "loop-file", "no")
			}
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	}
	return m, nil
}

func (m model) View() tea.View {
	vol, _ := m.mpv.GetVolume()
	volStr := strconv.FormatFloat(vol, 'f', 0, 64)
	spacer := strings.Repeat("\t", 9)
	percent := (m.timePosFlag / m.durationFlag)
	keybinds := m.data["Keybinds"].(map[string]any)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.spinner.View()+"torch-song",
		"current song: "+m.title,
		"",
		m.timePos+"/"+m.duration+spacer+"Volume: "+volStr,
		m.paused+"  "+m.progress.ViewAs(percent)+" "+m.repeatFlag,
		"",
		keybinds["prev"].(string)+" -> prev | "+keybinds["next"].(string)+" -> next | "+keybinds["vol-down"].(string)+"/"+keybinds["vol-up"].(string)+" -> adjust volume | "+keybinds["play"].(string)+" -> play | "+keybinds["mute"].(string)+" -> mute | "+
			keybinds["shuffle"].(string)+" -> shuffle | "+keybinds["repeat"].(string)+" -> repeat",
	)

	v := tea.NewView(container.Render(content))
	v.AltScreen = true

	return v
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to access user home dir: %v", err)
	}

	configDir := os.Getenv("TORCHSONG_CONFIG_DIR")
	if configDir == "" {
		configDir, err = os.UserConfigDir()
		if err != nil {
			log.Fatalf("config directory doesnt exist: %v", err)
		}
	}
	tsConfigDir := filepath.Join(configDir, "torchsong")
	exists, err := internal.DirExists(tsConfigDir)
	if err != nil {
		log.Fatalf("failed to read torch song dir: %v", err)
	}
	if !exists {
		log.Fatal("no config file to be read")
	}
	configPath := filepath.Join(configDir, "torchsong", "config.toml")
	config := make(map[string]any)
	_, err = toml.DecodeFile(configPath, &config)
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	Dir := config["Directory"].(map[string]any)
	music := Dir["path"].(string)

	filePath := filepath.Join(homeDir, music)
	socketPath := "/tmp/mpv.sock"
	server, client, err := gompv.StartAndConnect(context.Background(), socketPath, filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Stop()
	defer client.Close()

	err = client.ObserveProperties("media-title", "time-pos", "pause", "duration", "playlist-pos")
	if err != nil {
		log.Printf("Failed to observe properties")
	}
	spin, prog := internal.InitSpinnerAndProgress()
	p := tea.NewProgram(model{mpv: client, spinner: spin, progress: prog, data: config})
	go func() {
		for ev := range client.Events() {
			p.Send(ev)
		}
	}()

	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}

}
