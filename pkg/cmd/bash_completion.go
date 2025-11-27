package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Billionders/boilr/pkg/boilr"
	"github.com/Billionders/boilr/pkg/util/exit"
	"github.com/Billionders/boilr/pkg/util/osutil"
	cli "github.com/spf13/cobra"
)

func configureBashCompletion() error {
	// Windows 不支持 Bash completion
	if runtime.GOOS == "windows" {
		return fmt.Errorf("bash completion is not available on Windows")
	}

	bashCompletionFilePath := filepath.Join(boilr.Configuration.ConfigDirPath, "completion.bash")

	if err := Root.GenBashCompletionFile(bashCompletionFilePath); err != nil {
		return err
	}

	if err := Root.GenBashCompletionFile(bashCompletionFilePath); err != nil {
		return err
	}

	homeDir, err := osutil.GetUserHomeDir()
	if err != nil {
		return err
	}

	bashrcPath := filepath.Join(homeDir, ".bashrc")

	f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	bashrcText := `
# Enables command-line completion for boilr
source %s
`

	bashrcText = fmt.Sprintf(bashrcText, filepath.Join("$HOME", boilr.ConfigDirPath, "completion.bash"))

	if _, err = f.WriteString(bashrcText); err != nil {
		return err
	}

	return nil
}

// ConfigureBashCompletion generates bash auto-completion script and installs it.
var ConfigureBashCompletion = &cli.Command{
	Hidden: true,
	Use:    "configure-bash-completion",
	Short:  "Configure bash the auto-completion",
	Run: func(c *cli.Command, _ []string) {
		// 在 Windows 上提示不支持
		if runtime.GOOS == "windows" {
			exit.GoodEnough("Bash completion is not supported on Windows platform. This command is only available on Unix-like systems (Linux, macOS).")
		}

		if err := configureBashCompletion(); err != nil {
			exit.Fatal(fmt.Errorf("configure-bash-completion: %s", err))
		}

		exit.OK("Successfully configured bash auto-completion")
	},
}
