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

// usageTemplate is the custom template that conditionally includes Arguments section
// Note: We cannot use nindent as it's a Cobra internal function, so we rely on the fact
// that the data is already properly formatted
// Note: This is a copy of the defaultUsageTemplate in cobra/command.go with the
// Argument section added. We may have to update the copy if cobra new features rely on
// changes in the defaultUsageTemplate.
var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .Annotations.args}}

Arguments:
{{.Annotations.args}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// SetCustomUsageTemplate sets the custom help template on a command.
// This template will display an Arguments section if the command has an "args" annotation.
// It uses [FLAGS] (uppercase) instead of [flags] for consistency with [ID].
func SetCustomUsageTemplate(cmd *cobra.Command) {
	cmd.SetUsageTemplate(usageTemplate)
}
