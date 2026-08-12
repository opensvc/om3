package commoncmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// ArgAnnotationKey is the key used for the args annotation on cobra commands
const ArgAnnotationKey = "args"

// CmdWithArg appends argument documentation to the command's args annotation.
// It handles line feeds, padding and indentation to format the text properly.
// The argument description will be displayed in the "Arguments:" section of the help output.
func CmdWithArg(cmd *cobra.Command, arg string) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}

	existing := cmd.Annotations[ArgAnnotationKey]
	if existing != "" {
		// Append with proper formatting
		cmd.Annotations[ArgAnnotationKey] = existing + "\n" + formatArgText(arg)
	} else {
		cmd.Annotations[ArgAnnotationKey] = formatArgText(arg)
	}
}

// formatArgText ensures the argument text is properly formatted with indentation
func formatArgText(arg string) string {
	// Trim leading/trailing whitespace
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}

	// Replace multiple newlines with single newlines
	arg = strings.ReplaceAll(arg, "\n\n", "\n")
	arg = strings.ReplaceAll(arg, "\r\n", "\n")

	// Indent each line with 2 spaces for proper alignment in help output
	lines := strings.Split(arg, "\n")
	for i, line := range lines {
		lines[i] = "  " + strings.TrimLeft(line, " ")
	}

	return strings.Join(lines, "\n")
}

// helpTemplate is the custom template that conditionally includes Arguments section
// and uses [FLAGS] (uppercase) instead of [flags] for consistency with [ID]
// Note: We cannot use nindent as it's a Cobra internal function, so we rely on the fact
// that the data is already properly formatted
const helpTemplate = `{{if .Long}}{{.Long}}

{{end}}Usage:{{if .Runnable}}
  {{.Use}}{{if or .HasAvailableLocalFlags .HasAvailableInheritedFlags}} [FLAGS]{{end}}{{if .HasAvailableSubCommands}} [command]{{end}}{{else}}
  {{.CommandPath}}{{end}}{{if .HasExample}}
Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}
{{end}}{{if .Annotations.args}}

Arguments:
{{.Annotations.args}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages}}{{end}}{{if .HasAvailableInheritedFlags}}
{{end}}`

// SetCustomHelpTemplate sets the custom help template on a command.
// This template will display an Arguments section if the command has an "args" annotation.
// It uses [FLAGS] (uppercase) instead of [flags] for consistency with [ID].
func SetCustomHelpTemplate(cmd *cobra.Command) {
	cmd.SetHelpTemplate(helpTemplate)
}
