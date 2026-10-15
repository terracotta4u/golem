package tool

// Builtins returns the tools the agent exposes.
func Builtins() []Tool {
	return []Tool{
		NewRead(),
		NewWrite(),
		NewEdit(),
		NewShell(),
	}
}

// Reserved reports whether name belongs to a builtin tool.
func Reserved(name string) bool {
	for _, t := range Builtins() {
		if t.Spec().Name == name {
			return true
		}
	}
	return false
}
