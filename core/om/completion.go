package om

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "generate completion script",
	Long: `To load completions:

Bash:

  $ source <(om completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ om completion bash > /etc/bash_completion.d/om
  # macOS:
  $ om completion bash > /usr/local/etc/bash_completion.d/om

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ om completion zsh > "${fpath[1]}/om"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ om completion fish | source

  # To load completions for each session, execute once:
  $ om completion fish > ~/.config/fish/completions/om.fish

PowerShell:

  PS> om completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> om completion powershell > om.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			_ = cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			_ = cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			_ = cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			_ = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	root.AddCommand(completionCmd)
}
