package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Colors for terminal output
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	Gray   = "\033[37m"
	Bold   = "\033[1m"
)

// Manager handles user interface and output formatting
type Manager struct {
	colors  bool
	verbose bool
}

// NewManager creates a new UI manager
func NewManager(colors, verbose bool) *Manager {
	return &Manager{
		colors:  colors,
		verbose: verbose,
	}
}

// Success prints a success message
func (m *Manager) Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if m.colors {
		fmt.Printf("%s✓%s %s\n", Green, Reset, message)
	} else {
		fmt.Printf("✓ %s\n", message)
	}
}

// Error prints an error message
func (m *Manager) Error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if m.colors {
		fmt.Printf("%s✗%s %s\n", Red, Reset, message)
	} else {
		fmt.Printf("✗ %s\n", message)
	}
}

// Warning prints a warning message
func (m *Manager) Warning(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if m.colors {
		fmt.Printf("%s⚠%s %s\n", Yellow, Reset, message)
	} else {
		fmt.Printf("⚠ %s\n", message)
	}
}

// Info prints an informational message
func (m *Manager) Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if m.colors {
		fmt.Printf("%sℹ%s %s\n", Blue, Reset, message)
	} else {
		fmt.Printf("ℹ %s\n", message)
	}
}

// Progress prints a progress message (only if verbose)
func (m *Manager) Progress(format string, args ...interface{}) {
	if !m.verbose {
		return
	}
	message := fmt.Sprintf(format, args...)
	if m.colors {
		fmt.Printf("%s⣾%s %s\n", Blue, Reset, message)
	} else {
		fmt.Printf("→ %s\n", message)
	}
}

// InfoIndented prints an indented info message
func (m *Manager) InfoIndented(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("  %s\n", message)
}

// Confirm asks the user for confirmation
func (m *Manager) Confirm(message string) error {
	fmt.Printf("%s [y/N]: ", message)
	
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		return fmt.Errorf("operation cancelled by user")
	}

	return nil
}

// ConfirmWithOptions asks the user for confirmation with custom options
func (m *Manager) ConfirmWithOptions(message string, options map[string]string) (string, error) {
	// Show options
	fmt.Printf("%s\n", message)
	var keys []string
	for key, desc := range options {
		fmt.Printf("  [%s] %s\n", key, desc)
		keys = append(keys, key)
	}
	fmt.Print("Choose: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	
	// Check if response is valid
	if _, exists := options[response]; !exists {
		return "", fmt.Errorf("invalid option: %s", response)
	}

	return response, nil
}

// Header prints a section header
func (m *Manager) Header(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if m.colors {
		fmt.Printf("\n%s%s=== %s ===%s\n", Bold, Blue, message, Reset)
	} else {
		fmt.Printf("\n=== %s ===\n", message)
	}
}

// Separator prints a visual separator
func (m *Manager) Separator() {
	if m.colors {
		fmt.Printf("%s%s%s\n", Gray, strings.Repeat("─", 50), Reset)
	} else {
		fmt.Println(strings.Repeat("-", 50))
	}
}

// Table represents a simple table for displaying data
type Table struct {
	headers []string
	rows    [][]string
	manager *Manager
}

// NewTable creates a new table
func (m *Manager) NewTable() *Table {
	return &Table{
		manager: m,
	}
}

// SetHeaders sets the table headers
func (t *Table) SetHeaders(headers ...string) {
	t.headers = headers
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	t.rows = append(t.rows, cells)
}

// Render renders the table to output
func (t *Table) Render() {
	if len(t.headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(t.headers))
	for i, header := range t.headers {
		widths[i] = len(header)
	}

	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print headers
	t.printRow(t.headers, widths, true)
	
	// Print separator
	separator := make([]string, len(t.headers))
	for i, width := range widths {
		separator[i] = strings.Repeat("─", width)
	}
	t.printRow(separator, widths, false)

	// Print rows
	for _, row := range t.rows {
		t.printRow(row, widths, false)
	}
}

// printRow prints a single table row
func (t *Table) printRow(cells []string, widths []int, isHeader bool) {
	var parts []string
	for i, cell := range cells {
		width := widths[i]
		if i < len(widths) {
			if isHeader && t.manager.colors {
				parts = append(parts, fmt.Sprintf("%s%-*s%s", Bold, width, cell, Reset))
			} else {
				parts = append(parts, fmt.Sprintf("%-*s", width, cell))
			}
		}
	}
	fmt.Printf("┌%s┐\n", strings.Join(parts, " │ "))
}

// ProgressBar represents a simple progress bar (placeholder for future enhancement)
type ProgressBar struct {
	total   int
	current int
	width   int
	manager *Manager
}

// NewProgressBar creates a new progress bar
func (m *Manager) NewProgressBar(total int) *ProgressBar {
	return &ProgressBar{
		total:   total,
		current: 0,
		width:   40,
		manager: m,
	}
}

// Update updates the progress bar
func (pb *ProgressBar) Update(current int) {
	pb.current = current
	pb.render()
}

// Increment increments the progress bar by 1
func (pb *ProgressBar) Increment() {
	pb.current++
	pb.render()
}

// render renders the progress bar
func (pb *ProgressBar) render() {
	if pb.total <= 0 {
		return
	}

	percent := float64(pb.current) / float64(pb.total)
	filled := int(percent * float64(pb.width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)
	
	if pb.manager.colors {
		fmt.Printf("\r%s[%s]%s %.1f%% (%d/%d)", 
			Blue, bar, Reset, percent*100, pb.current, pb.total)
	} else {
		fmt.Printf("\r[%s] %.1f%% (%d/%d)", 
			bar, percent*100, pb.current, pb.total)
	}

	if pb.current >= pb.total {
		fmt.Println() // New line when complete
	}
}

// Finish completes the progress bar
func (pb *ProgressBar) Finish() {
	pb.Update(pb.total)
}

// SetMessage sets a custom message for the progress bar
func (pb *ProgressBar) SetMessage(message string) {
	pb.render()
	if pb.manager.colors {
		fmt.Printf(" %s%s%s", Cyan, message, Reset)
	} else {
		fmt.Printf(" %s", message)
	}
}

// Spinner represents a spinning progress indicator
type Spinner struct {
	message  string
	chars    []string
	index    int
	active   bool
	manager  *Manager
	stopChan chan bool
}

// NewSpinner creates a new spinner
func (m *Manager) NewSpinner(message string) *Spinner {
	return &Spinner{
		message:  message,
		chars:    []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		index:    0,
		active:   false,
		manager:  m,
		stopChan: make(chan bool),
	}
}

// Start starts the spinner
func (s *Spinner) Start() {
	s.active = true
	go s.spin()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	if s.active {
		s.active = false
		s.stopChan <- true
		fmt.Print("\r\033[K") // Clear line
	}
}

// UpdateMessage updates the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.message = message
}

// spin runs the spinning animation
func (s *Spinner) spin() {
	for s.active {
		select {
		case <-s.stopChan:
			return
		default:
			char := s.chars[s.index%len(s.chars)]
			if s.manager.colors {
				fmt.Printf("\r%s%s%s %s", Blue, char, Reset, s.message)
			} else {
				fmt.Printf("\r%s %s", char, s.message)
			}
			s.index++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// SuccessStop stops the spinner with a success message
func (s *Spinner) SuccessStop(message string) {
	s.Stop()
	s.manager.Success("%s", message)
}

// ErrorStop stops the spinner with an error message
func (s *Spinner) ErrorStop(message string) {
	s.Stop()
	s.manager.Error("%s", message)
}

// MultiStepProgress represents a multi-step progress indicator
type MultiStepProgress struct {
	steps    []string
	current  int
	manager  *Manager
	statuses []string // "pending", "running", "completed", "failed"
}

// NewMultiStepProgress creates a new multi-step progress indicator
func (m *Manager) NewMultiStepProgress(steps []string) *MultiStepProgress {
	statuses := make([]string, len(steps))
	for i := range statuses {
		statuses[i] = "pending"
	}
	return &MultiStepProgress{
		steps:    steps,
		current:  0,
		manager:  m,
		statuses: statuses,
	}
}

// StartStep starts a specific step
func (msp *MultiStepProgress) StartStep(index int) {
	if index < len(msp.statuses) {
		msp.current = index
		msp.statuses[index] = "running"
		msp.render()
	}
}

// CompleteStep marks a step as completed
func (msp *MultiStepProgress) CompleteStep(index int) {
	if index < len(msp.statuses) {
		msp.statuses[index] = "completed"
		msp.render()
	}
}

// FailStep marks a step as failed
func (msp *MultiStepProgress) FailStep(index int) {
	if index < len(msp.statuses) {
		msp.statuses[index] = "failed"
		msp.render()
	}
}

// render displays the multi-step progress
func (msp *MultiStepProgress) render() {
	fmt.Println() // New line
	for i, step := range msp.steps {
		var icon, color string
		switch msp.statuses[i] {
		case "pending":
			icon, color = "○", Gray
		case "running":
			icon, color = "●", Blue
		case "completed":
			icon, color = "✓", Green
		case "failed":
			icon, color = "✗", Red
		}

		if msp.manager.colors {
			fmt.Printf("  %s%s%s %s\n", color, icon, Reset, step)
		} else {
			fmt.Printf("  %s %s\n", icon, step)
		}
	}
}

// ColorString applies color to a string if colors are enabled
func (m *Manager) ColorString(text, color string) string {
	if !m.colors {
		return text
	}
	return color + text + Reset
}

// Helper methods for colored strings
func (m *Manager) Red(text string) string    { return m.ColorString(text, Red) }
func (m *Manager) Green(text string) string  { return m.ColorString(text, Green) }
func (m *Manager) Yellow(text string) string { return m.ColorString(text, Yellow) }
func (m *Manager) Blue(text string) string   { return m.ColorString(text, Blue) }
func (m *Manager) Purple(text string) string { return m.ColorString(text, Purple) }
func (m *Manager) Cyan(text string) string   { return m.ColorString(text, Cyan) }
func (m *Manager) Gray(text string) string   { return m.ColorString(text, Gray) }
func (m *Manager) Bold(text string) string   { return m.ColorString(text, Bold) }