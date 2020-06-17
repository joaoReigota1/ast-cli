package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/checkmarxDev/ast-cli/internal/wrappers"

	commonParams "github.com/checkmarxDev/ast-cli/internal/params"
	"github.com/spf13/viper"

	"gopkg.in/yaml.v2"

	"github.com/checkmarxDev/ast-cli/internal/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	configFileFlag      = "config"
	installationDirFlag = "installation-dir"
	updateDirFlag       = "update-dir"
	roleFlag            = "role"
)

var (
	astRoleFlagUsage = fmt.Sprintf("The runtime role. Available roles are: %s",
		strings.Join([]string{
			commonParams.ScaAgent,
			commonParams.SastALlInOne,
			commonParams.SastManager,
			commonParams.SastEngine}, ","))
)

func NewSingleNodeCommand(healthCheckWrapper wrappers.HealthCheckWrapper, defaultConfigFileLocation string) *cobra.Command {
	singleNodeCmd := &cobra.Command{
		Use:   "single-node",
		Short: "Single Node",
	}

	upSingleNodeCmd := &cobra.Command{
		Use:   "up",
		Short: "Start the system",
		RunE:  runUpSingleNodeCommand(defaultConfigFileLocation),
	}
	downSingleNodeCmd := &cobra.Command{
		Use:   "down",
		Short: "Stop the system",
		RunE:  runDownSingleNodeCommand(),
	}

	updateSingleNodeCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the system",
		RunE:  runUpdateSingleNodeCommand(defaultConfigFileLocation),
	}

	healthSingleNodeCmd := NewHealthCheckCommand(healthCheckWrapper)

	defaultDir := "./"
	installationConfigFileUsage := "Configuration file path (optional)"
	installationDirUsage := "Installation dir path"
	updateDirUsage := "Update dir path. The dir where the updated artifacts reside"

	upSingleNodeCmd.PersistentFlags().String(configFileFlag, "", installationConfigFileUsage)
	upSingleNodeCmd.PersistentFlags().String(installationDirFlag, defaultDir, installationDirUsage)
	upSingleNodeCmd.PersistentFlags().String(roleFlag, "", astRoleFlagUsage)
	// Binding the AST_ROLE env var to the --role flag
	_ = viper.BindPFlag(commonParams.AstRoleKey, upSingleNodeCmd.PersistentFlags().Lookup(roleFlag))

	downSingleNodeCmd.PersistentFlags().String(installationDirFlag, defaultDir, installationDirUsage)

	updateSingleNodeCmd.PersistentFlags().String(updateDirFlag, defaultDir, updateDirUsage)
	updateSingleNodeCmd.PersistentFlags().String(configFileFlag, "", installationConfigFileUsage)

	healthSingleNodeCmd.PersistentFlags().String(roleFlag, commonParams.ScaAgent, astRoleFlagUsage)
	_ = viper.BindPFlag(commonParams.AstRoleKey, healthSingleNodeCmd.PersistentFlags().Lookup(roleFlag))

	singleNodeCmd.AddCommand(
		upSingleNodeCmd,
		downSingleNodeCmd,
		healthSingleNodeCmd,
		updateSingleNodeCmd)
	return singleNodeCmd
}

func runUpSingleNodeCommand(defaultConfigFileLocation string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		installationDir, _ := cmd.Flags().GetString(installationDirFlag)
		return runUp(cmd, defaultConfigFileLocation, installationDir)
	}
}

func runUp(cmd *cobra.Command, defaultConfigFileLocation, scriptsRootFolder string) error {
	writeToStandardOutput("Trying to start...")
	err := runUpScript(cmd, defaultConfigFileLocation, scriptsRootFolder)
	if err != nil {
		msg := "Failed to start"
		return errors.Wrapf(err, msg)
	}
	writeToStandardOutput("System is up!")
	return nil
}

func runDownSingleNodeCommand() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		writeToStandardOutput("Trying to stop...")
		err := runDownScript(cmd)

		if err != nil {
			msg := "Failed to stop"
			return errors.Wrapf(err, msg)
		}
		writeToStandardOutput("System is down!")
		return nil
	}
}

func runUpdateSingleNodeCommand(defaultConfigFileLocation string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := runDownSingleNodeCommand()(cmd, args)
		if err != nil {
			return err
		}
		updateDir, _ := cmd.Flags().GetString(updateDirFlag)
		err = runUp(cmd, defaultConfigFileLocation, updateDir)
		if err != nil {
			return err
		}
		writeToStandardOutput("System updated successfully!")
		return nil
	}
}

func runUpScript(cmd *cobra.Command, defaultConfigFileLocation, scriptsRootFolder string) error {
	upScriptPath := getScriptPathRelativeToInstallation("up.sh", cmd)
	role := viper.GetString(commonParams.AstRoleKey)

	err := runWithConfig(cmd, upScriptPath, role, defaultConfigFileLocation, scriptsRootFolder)
	if err != nil {
		return errors.Wrapf(err, "Failed to run up script")
	}
	return nil
}

func runDownScript(cmd *cobra.Command) error {
	var err error
	downScriptPath := getScriptPathRelativeToInstallation("down.sh", cmd)

	installationDir, _ := cmd.Flags().GetString(installationDirFlag)
	envs := []string{
		envKeyAndValue(astInstallationPathEnv, installationDir),
	}

	_, _, err = runBashCommand(downScriptPath, envs)
	if err != nil {
		return errors.Wrapf(err, "Failed to run down script")
	}
	return nil
}

func runWithConfig(cmd *cobra.Command, scriptPath, role, defaultConfigFileLocation, scriptsRootFolder string) error {
	var err error
	configuration := &config.SingleNodeConfiguration{}

	configFile, _ := cmd.Flags().GetString(configFileFlag)

	// Give precedence to the the config flag
	// We have been provided with a config file
	if configFile != "" {
		configuration, err = tryLoadConfiguration(configFile)
		if err != nil {
			return err
		}
	} else {
		// Try to run with the default config file
		if _, err = os.Stat(defaultConfigFileLocation); err == nil {
			PrintIfVerbose(fmt.Sprintf("Reading configuration from default location at %s", defaultConfigFileLocation))
			configuration, err = tryLoadConfiguration(defaultConfigFileLocation)
			if err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			PrintIfVerbose(fmt.Sprintf("No configuration file provided via flag and no configutaion file was found at %s. "+
				"Proceeding with default configuration.", defaultConfigFileLocation))
		}
	}

	envVars := createEnvVarsForCommand(configuration, scriptsRootFolder, role)

	_, _, err = runBashCommand(scriptPath, envVars)
	return err
}

func tryLoadConfiguration(configFile string) (*config.SingleNodeConfiguration, error) {
	configuration := config.SingleNodeConfiguration{}

	PrintIfVerbose(fmt.Sprintf("Trying to load configuration from file %s", configFile))
	configInput, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to load config file")
	}

	err = yaml.Unmarshal(configInput, &configuration)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to parse configuration file")
	}
	return &configuration, nil
}

func writeToStandardOutput(msg string) {
	fmt.Fprintln(os.Stdout, msg)
}

func getPathRelativeToInstallation(filePath string, cmd *cobra.Command) string {
	installationDir, _ := cmd.Flags().GetString(installationDirFlag)
	return path.Join(installationDir, filePath)
}

func getScriptPathRelativeToInstallation(scriptFile string, cmd *cobra.Command) string {
	scriptsDir := ".scripts"
	return getPathRelativeToInstallation(path.Join(scriptsDir, scriptFile), cmd)
}
