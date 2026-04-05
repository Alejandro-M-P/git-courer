package ui

// Port defines what UI operations the core can do
// The core uses this interface, the adapter implements it
type Port interface {
	// Confirm asks the user for confirmation
	// Returns true if confirmed, false if cancelled
	Confirm(message string) (bool, error)

	// Edit allows user to edit text
	Edit(initial string) (string, error)

	// ShowError displays an error message
	ShowError(message string) error

	// ShowInfo displays an info message
	ShowInfo(message string) error

	// Select presents options and returns the selected one
	Select(message string, options []string) (int, error)
}
