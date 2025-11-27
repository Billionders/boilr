package boilr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Billionders/boilr/pkg/util/exit"
	"github.com/Billionders/boilr/pkg/util/osutil"
)

const (
	// AppName of the application
	AppName = "boilr"

	// Version of the application
	Version = "0.3.0"

	// ConfigDirPath is the configuration directory of the application
	ConfigDirPath = ".config/boilr"

	// ConfigFileName is the configuration file name of the application
	ConfigFileName = "config.json"

	// TemplateDir is the directory that contains the template registry
	TemplateDir = "templates"

	// ContextFileName is the name of the file that contains the context values for the template
	ContextFileName = "project.json"

	// TemplateDirName is the name of the directory that contains the template files in a boilr template
	TemplateDirName = "template"

	// TemplateMetadataName is the name of the file that contains the metadata about the template saved in registry
	TemplateMetadataName = "__metadata.json"

	// GithubOwner is the owner of the github repository
	GithubOwner = "Billionders"

	// GithubRepo is the name of the github repository
	GithubRepo = "boilr"
)

// Configuration contains the values for needed for boilr to operate.
// These values can be overridden by the inclusion of a boilr.json
// file in the configuration directory.
var Configuration = struct {
	FilePath        string
	ConfigDirPath   string
	TemplateDirPath string
}{}

// TemplatePath returns the absolute path of a template given the name of the template.
func TemplatePath(name string) (string, error) {
	return filepath.Join(Configuration.TemplateDirPath, name), nil
}

func IsTemplateDirInitialized() (bool, error) {
	return osutil.DirExists(Configuration.TemplateDirPath)
}

// getUserHomeDir 跨平台获取用户主目录
func getUserHomeDir() (string, error) {
	if runtime.GOOS == "windows" {
		// Windows 系统使用 USERPROFILE
		homeDir := os.Getenv("USERPROFILE")
		if homeDir == "" {
			return "", fmt.Errorf("environment variable USERPROFILE is not set")
		}
		return homeDir, nil
	}

	// Unix/Linux/macOS 系统使用 HOME
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return "", fmt.Errorf("environment variable HOME is not set")
	}
	return homeDir, nil
}

func init() {
	homeDir, err := getUserHomeDir()
	if err != nil {
		exit.Error(err)
	}

	Configuration.FilePath = filepath.Join(homeDir, ConfigDirPath, ConfigFileName)
	Configuration.ConfigDirPath = filepath.Join(homeDir, ConfigDirPath)
	Configuration.TemplateDirPath = filepath.Join(homeDir, ConfigDirPath, TemplateDir)

	// Read .config/boilr/config.json if exists
	// TODO use defaults if config.json doesn't exist
	hasConfig, err := osutil.FileExists(Configuration.FilePath)
	if err != nil {
		exit.Error(err)
	}

	if !hasConfig {
		// TODO report the absence of config.json
		//tlog.Debug("Couldn't find %s user configuration", ConfigFileName)
		return
	}

	buf, err := os.ReadFile(Configuration.FilePath)
	if err != nil {
		exit.Error(err)
	}

	if err := json.Unmarshal(buf, &Configuration); err != nil {
		exit.Error(err)
	}
}
