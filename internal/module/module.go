package module

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"

	"github.com/commitdev/zero/internal/config"
	"github.com/commitdev/zero/internal/config/moduleconfig"
	"github.com/commitdev/zero/internal/constants"
	"github.com/commitdev/zero/internal/util"
	"github.com/hashicorp/go-getter"
	"github.com/manifoldco/promptui"
)

// TemplateModule merges a module instance params with the static configs
type TemplateModule struct {
	config.ModuleInstance // @TODO Move this
	Config                moduleconfig.ModuleConfig
}

// FetchModule downloads the remote module source (or loads the local files) and parses the module config yaml
func FetchModule(source string) (moduleconfig.ModuleConfig, error) {
	config := moduleconfig.ModuleConfig{}
	localPath := GetSourceDir(source)
	if !isLocal(source) {
		err := getter.Get(localPath, source)
		if err != nil {
			return config, err
		}
	}

	configPath := path.Join(localPath, constants.ZeroModuleYml)
	config, err := moduleconfig.LoadModuleConfig(configPath)
	return config, err
}

func appendProjectEnvToCmdEnv(envMap map[string]string, envList []string) []string {
	for key, val := range envMap {
		if val != "" {
			envList = append(envList, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return envList
}

// aws cli prints output with linebreak in them
func sanitizePromptResult(str string) string {
	re := regexp.MustCompile("\\n")
	return re.ReplaceAllString(str, "")
}

// TODO : Use this function signature instead
// PromptParams renders series of prompt UI based on the config
func PromptParams(moduleConfig moduleconfig.ModuleConfig, parameters map[string]string) (map[string]string, error) {
	return map[string]string{}, nil
}

// PromptParams renders series of prompt UI based on the config
func (m *TemplateModule) PromptParams(projectContext map[string]string) error {
	for _, promptConfig := range m.Config.Parameters {

		label := promptConfig.Label
		if promptConfig.Label == "" {
			label = promptConfig.Field
		}

		// deduplicate fields already prompted and received
		if _, isAlreadySet := projectContext[promptConfig.Field]; isAlreadySet {
			continue
		}

		var err error
		var result string
		if len(promptConfig.Options) > 0 {
			prompt := promptui.Select{
				Label: label,
				Items: promptConfig.Options,
			}
			_, result, err = prompt.Run()

		} else if promptConfig.Execute != "" {
			// TODO: this could perhaps be set as a default for part of regular prompt
			cmd := exec.Command("bash", "-c", promptConfig.Execute)
			cmd.Env = appendProjectEnvToCmdEnv(projectContext, os.Environ())
			out, err := cmd.Output()

			if err != nil {
				log.Fatalf("Failed to execute  %v\n", err)
				panic(err)
			}
			result = string(out)
		} else {
			prompt := promptui.Prompt{
				Label: label,
			}
			result, err = prompt.Run()
		}
		if err != nil {
			return err
		}

		result = sanitizePromptResult(result)
		if m.Params == nil {
			m.Params = make(map[string]string)
		}
		m.Params[promptConfig.Field] = result
		projectContext[promptConfig.Field] = result
	}

	return nil
}

// GetSourcePath gets a unique local source directory name. For local modules, it use the local directory
func GetSourceDir(source string) string {
	if !isLocal(source) {
		h := md5.New()
		io.WriteString(h, source)
		source = base64.StdEncoding.EncodeToString(h.Sum(nil))
		return path.Join(constants.TemplatesDir, source)
	} else {
		return source
	}
}

// IsLocal uses the go-getter FileDetector to check if source is a file
func isLocal(source string) bool {
	pwd := util.GetCwd()

	// ref: https://github.com/hashicorp/go-getter/blob/master/detect_test.go
	out, err := getter.Detect(source, pwd, getter.Detectors)

	match, err := regexp.MatchString("^file://.*", out)
	if err != nil {
		log.Panicf("invalid source format %s", err)
	}

	return match
}

func withPWD(pwd string) func(*getter.Client) error {
	return func(c *getter.Client) error {
		c.Pwd = pwd
		return nil
	}
}