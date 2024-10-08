package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const snippetsFile = "snippets.txt"

var (
	titleStyle = lipgloss.NewStyle().
			MarginLeft(2).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color("#FAFAFA"))

	selectedItemStyle = itemStyle.
				Foreground(lipgloss.Color("#7D56F4"))

	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle       = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)

	quitTextStyle = lipgloss.NewStyle().Margin(1, 0, 2, 4)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#BDBDBD"))
)

type snippet struct {
	ID       int
	Name     string
	Language string
	Code     string
}

type item string

func (i item) FilterValue() string { return string(i) }
func (i item) Title() string       { return string(i) }
func (i item) Description() string { return "" }

type model struct {
	snippets     []snippet
	state        string
	input        textinput.Model
	textarea     textarea.Model
	currentField int
	newSnippet   snippet
	selectedItem int
	err          error
	list         list.Model
	width        int
	height       int
	logger       *log.Logger
}

func initialModel() (model, error) {
	items := []list.Item{
		item("View Snippets"),
		item("Add Snippet"),
		item("Delete Snippet"),
		item("Quit"),
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Snippet Manager"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	ti := textinput.New()
	ti.PlaceholderStyle = placeholderStyle
	ti.TextStyle = inputStyle

	ta := textarea.New()
	ta.Placeholder = "Enter snippet code"
	ta.CharLimit = 0
	ta.ShowLineNumbers = true
	ta.Prompt = "|"
	ta.SetWidth(40)
	ta.SetHeight(10)

	// Set up logger
	logFile, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return model{}, fmt.Errorf("failed to open log file: %v", err)
	}

	logger := log.New(logFile, "", log.LstdFlags)

	return model{
		snippets: loadSnippets(),
		state:    "menu",
		input:    ti,
		textarea: ta,
		list:     l,
		logger:   logger,
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Add logging
		m.logger.Printf("Key pressed: %s, Current state: %s\n", msg.String(), m.state)

		// Handle Esc key globally
		if msg.Type == tea.KeyEsc {
			m.logger.Println("Esc key pressed. Handling...")
			switch m.state {
			case "menu":
				// In menu, Esc does nothing
				m.logger.Println("In menu, Esc does nothing")
			default:
				// In other states, Esc should return to menu
				m.logger.Println("Returning to menu due to Esc")
				return m.resetState(), nil
			}
		}

		if msg.String() == "q" {
			m.logger.Println("Quitting application due to 'q' key")
			return m, tea.Quit
		}
		switch m.state {
		case "menu":
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			if msg.Type == tea.KeyEnter {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					switch string(i) {
					case "View Snippets":
						m.state = "view"
					case "Add Snippet":
						m.state = "add"
						m.currentField = 0
						m.newSnippet = snippet{}
						m.input.Placeholder = "Name"
						m.input.SetValue("")
						m.input.Focus()
					case "Delete Snippet":
						m.state = "delete"
						m.selectedItem = 0
					case "Quit":
						return m, tea.Quit
					}
				}
			}
		case "add":
			switch msg.Type {
			case tea.KeyEnter:
				if m.currentField < 2 {
					switch m.currentField {
					case 0:
						m.newSnippet.Name = m.input.Value()
						m.input.SetValue("")
						m.input.Placeholder = "Language"
						m.currentField++
					case 1:
						m.newSnippet.Language = m.input.Value()
						m.input.SetValue("")
						m.textarea.Focus()
						m.currentField++
					}
				}
				// If we're in the textarea, let it handle the Enter key
			case tea.KeyCtrlS:
				if m.currentField == 2 {
					// Submit the snippet
					m.newSnippet.Code = m.textarea.Value()
					m.newSnippet.ID = generateID(m.snippets)
					m.snippets = append(m.snippets, m.newSnippet)
					saveSnippets(m.snippets)
					return m.resetState(), nil
				}
			}
		case "delete":
			if msg.Type == tea.KeyEnter {
				if m.selectedItem >= 0 && m.selectedItem < len(m.snippets) {
					m.snippets = append(m.snippets[:m.selectedItem], m.snippets[m.selectedItem+1:]...)
					saveSnippets(m.snippets)
				}
				m.state = "menu"
				m.selectedItem = 0
			} else if msg.String() == "up" && m.selectedItem > 0 {
				m.selectedItem--
			} else if msg.String() == "down" && m.selectedItem < len(m.snippets)-1 {
				m.selectedItem++
			}
		case "view":
			// No additional handling needed here, Esc is handled globally
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.state == "add" {
		if m.currentField < 2 {
			m.input, cmd = m.input.Update(msg)
		} else {
			m.textarea, cmd = m.textarea.Update(msg)
		}
	}
	return m, cmd
}

func (m model) View() string {
	switch m.state {
	case "menu":
		return m.list.View()
	case "view":
		var s strings.Builder
		s.WriteString(titleStyle.Render("View Snippets"))
		s.WriteString("\n\n")
		for _, snip := range m.snippets {
			s.WriteString(itemStyle.Render(fmt.Sprintf("ID: %d\nName: %s\nLanguage: %s\nCode:\n", snip.ID, snip.Name, snip.Language)))

			// Split the code into lines and render each line
			codeLines := strings.Split(snip.Code, "\n")
			for _, line := range codeLines {
				s.WriteString(itemStyle.Render(line + "\n"))
			}

			s.WriteString(itemStyle.Render("----------------------\n"))
		}
		s.WriteString(quitTextStyle.Render("Press 'esc' to return to menu"))
		return s.String()
	case "add":
		var s strings.Builder
		s.WriteString(titleStyle.Render("Add Snippet"))
		s.WriteString("\n\n")
		prompt := ""
		switch m.currentField {
		case 0:
			prompt = "Enter snippet name"
			s.WriteString(itemStyle.Render(fmt.Sprintf("%s:\n%s\n", prompt, m.input.View())))
		case 1:
			prompt = "Enter snippet language"
			s.WriteString(itemStyle.Render(fmt.Sprintf("%s:\n%s\n", prompt, m.input.View())))
		case 2:
			prompt = "Enter snippet code"
			s.WriteString(itemStyle.Render(fmt.Sprintf("%s:\n%s\n", prompt, m.textarea.View())))
			s.WriteString(quitTextStyle.Render("(Press Ctrl+S to save, Esc to cancel)"))
		}
		s.WriteString("\n")
		return s.String()
	case "delete":
		var s strings.Builder
		s.WriteString(titleStyle.Render("Delete Snippet"))
		s.WriteString("\n\n")

		maxID := 0
		for _, snip := range m.snippets {
			if snip.ID > maxID {
				maxID = snip.ID
			}
		}
		idWidth := len(strconv.Itoa(maxID))

		for i, snip := range m.snippets {
			style := itemStyle
			if m.selectedItem == i {
				style = selectedItemStyle
			}
			formattedLine := fmt.Sprintf("%-*d: %s", idWidth, snip.ID, snip.Name)
			s.WriteString(style.Render(formattedLine) + "\n")
		}
		s.WriteString("\n")
		s.WriteString(quitTextStyle.Render("Use arrow keys to select, Enter to delete, 'esc' to cancel"))
		return s.String()
	default:
		return "Unknown state"
	}
}

func (m model) resetState() model {
	m.state = "menu"
	m.currentField = 0
	m.newSnippet = snippet{}
	m.input.SetValue("")
	m.textarea.SetValue("")
	m.input.Placeholder = "Name"
	return m
}

func main() {
	initialModel, err := initialModel()
	if err != nil {
		fmt.Println("Error initializing model:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

func loadSnippets() []snippet {
	file, err := os.Open(snippetsFile)
	if err != nil {
		return []snippet{}
	}
	defer file.Close()

	var snippets []snippet
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "|||")
		if len(parts) == 4 {
			id, _ := strconv.Atoi(parts[0])
			decodedCode, _ := base64.StdEncoding.DecodeString(parts[3])
			snippets = append(snippets, snippet{
				ID:       id,
				Name:     parts[1],
				Language: parts[2],
				Code:     string(decodedCode),
			})
		}
	}
	return snippets
}

func saveSnippets(snippets []snippet) {
	file, err := os.Create(snippetsFile)
	if err != nil {
		fmt.Println("Error saving snippets:", err)
		return
	}
	defer file.Close()

	for _, s := range snippets {
		// Encode the code as base64 to preserve newlines
		encodedCode := base64.StdEncoding.EncodeToString([]byte(s.Code))
		fmt.Fprintf(file, "%d|||%s|||%s|||%s\n", s.ID, s.Name, s.Language, encodedCode)
	}
}

func generateID(snippets []snippet) int {
	maxID := 0
	for _, s := range snippets {
		if s.ID > maxID {
			maxID = s.ID
		}
	}
	return maxID + 1
}
