package dast

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/MakeNowJust/heredoc"
	"github.com/checkmarx/ast-cli/internal/commands/util"
	"github.com/checkmarx/ast-cli/internal/logger"
	commonParams "github.com/checkmarx/ast-cli/internal/params"
	"github.com/checkmarx/ast-cli/internal/wrappers"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type dastScanType int64

const (
	dastContainerPrefixName    = "cli-dast-realtime-"
	containerFolderRemoving    = "Removing folder in temp"
	containerTempDirPattern    = "dast"
	containerCreateFolderError = "Error creating temporary directory"
	containerFileSourceMissing = "--file is required for dast-realtime command"
	containerFileSourceError   = " Error reading file"
	containerWriteFolderError  = " Error writing file to temporary directory"
	containerVolumeFormat      = "%s:/path"
	containerStarting          = "Starting dast container"
	containerFormatInfo        = "The report format and output path cannot be overridden."
	invalidEngineError         = "executable file not found in $PATH"
	invalidEngineMessage       = "Please verify if engine is installed and running"
	containerScanPath          = "/path"

	// container args
	containerRun        = "run"
	containerRemove     = "--rm"
	containerVolumeFlag = "-v"
	containerNameFlag   = "--name"
	containerImage      = "dast:cli"

	containerWebScan = "web"
	containerAPIScan = "api"

	containerOpenAPIFile = "--openapi"
	containerPathFile    = "--path"
	containerOutput      = "--output"
	containerTimeout     = "--timeout"
	containerUpdate      = "--update-interval"
	containerJVMProperty = "--jvm-properties"
	containerLogLevel    = "--log-level"

	// api errors
	failedToSendResults = "Failed to send results"

	dastWebScan dastScanType = iota
	dastAPIScan
)

func ScanDastRealTimeAPISubCommand(dastWrapper wrappers.DastResultsWrapper) *cobra.Command {
	dastContainerID := uuid.New()
	viper.Set(commonParams.DastContainerNameKey, dastContainerPrefixName+dastContainerID.String())
	dastRealtTimeAPIScanCmd := &cobra.Command{
		Use:   "dast-realtime-api",
		Short: "Create and run dast API scan",
		Long:  "The dast-realtime command enables the ability to create, run and retrieve results from a dast API scan using a docker image.",
		Example: heredoc.Doc(
			`$ cx scan dast-realtime --file <file>  --engine <engine> --open-api <open-api>`,
		),
		RunE: runDastRealTime(dastAPIScan, dastWrapper),
	}

	// DAST Flags
	dastRealtTimeAPIScanCmd.PersistentFlags().String(
		commonParams.DastRealTimeFile,
		"",
		"Path to the dast configuration file")
	dastRealtTimeAPIScanCmd.PersistentFlags().String(
		commonParams.DastOpenAPIFile,
		"",
		"Path to the open API specification file")
	dastRealtTimeAPIScanCmd.PersistentFlags().String(
		commonParams.DastOutputFile,
		"",
		"Path to the output file")
	dastRealtTimeAPIScanCmd.PersistentFlags().String(
		commonParams.DastRealTimeEngine,
		"docker",
		"Name in the $PATH for the container engine to run dast. Example:podman.",
	)

	util.MarkFlagAsRequired(dastRealtTimeAPIScanCmd, commonParams.DastRealTimeFile)
	return dastRealtTimeAPIScanCmd
}

func ScanDastRealTimeWebSubCommand(dastWrapper wrappers.DastResultsWrapper) *cobra.Command {
	dastContainerID := uuid.New()
	viper.Set(commonParams.DastContainerNameKey, dastContainerPrefixName+dastContainerID.String())
	dastRealtTimeWebScanCmd := &cobra.Command{
		Use:   "dast-realtime-web",
		Short: "Create and run dast Web scan",
		Long:  "The dast-realtime command enables the ability to create, run and retrieve results from a dast Web scan using a docker image.",
		Example: heredoc.Doc(
			`$ cx scan dast-realtime --file <file>  --engine <engine>`,
		),
		RunE: runDastRealTime(dastWebScan, dastWrapper),
	}

	// DAST Flags
	dastRealtTimeWebScanCmd.PersistentFlags().String(
		commonParams.DastRealTimeFile,
		"",
		"Path to the dast configuration file")
	dastRealtTimeWebScanCmd.PersistentFlags().String(
		commonParams.DastOutputFile,
		"",
		"Path to the output file")
	dastRealtTimeWebScanCmd.PersistentFlags().String(
		commonParams.DastRealTimeEngine,
		"docker",
		"Name in the $PATH for the container engine to run dast. Example:podman.",
	)

	util.MarkFlagAsRequired(dastRealtTimeWebScanCmd, commonParams.DastRealTimeFile)
	return dastRealtTimeWebScanCmd
}

func runDastRealTime(scanType dastScanType, dastWrapper wrappers.DastResultsWrapper) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		volumeMap, tempDir, err := createDastScanEnv(cmd)
		if err != nil {
			return errors.Errorf("%s", err)
		}

		err = runDastScan(cmd, volumeMap, tempDir, scanType, dastWrapper)
		if err != nil {
			// Removing temporary dir
			logger.PrintIfVerbose(containerFolderRemoving)
			os.RemoveAll(tempDir)
			return errors.Errorf("%s", err)
		}

		logger.PrintIfVerbose(containerFolderRemoving)
		os.RemoveAll(tempDir)
		return nil
	}
}

func createDastScanEnv(cmd *cobra.Command) (volumeMap, dastDir string, err error) {
	dastDir, err = ioutil.TempDir("", containerTempDirPattern)
	if err != nil {
		return "", "", errors.New(containerCreateFolderError)
	}

	dastFilePath, _ := cmd.Flags().GetString(commonParams.DastRealTimeFile)
	if len(dastFilePath) < 1 {
		return "", "", errors.New(containerFileSourceMissing)
	}

	dastOpenAPIFilePath, _ := cmd.Flags().GetString(commonParams.DastOpenAPIFile)

	dastFile, err := ioutil.ReadFile(dastFilePath)
	if err != nil {
		return "", "", errors.New(containerFileSourceError)
	}

	_, file := filepath.Split(dastFilePath)
	destinationFile := fmt.Sprintf("%s/%s", dastDir, file)
	err = ioutil.WriteFile(destinationFile, dastFile, 0666)
	if err != nil {
		return "", "", errors.New(containerWriteFolderError)
	}

	if len(dastOpenAPIFilePath) > 0 {
		var dastOpenAPIFile []byte
		dastOpenAPIFile, err = ioutil.ReadFile(dastOpenAPIFilePath)
		if err != nil {
			return "", "", errors.New(containerFileSourceError)
		}
		_, file = filepath.Split(dastOpenAPIFilePath)
		destinationFile = fmt.Sprintf("%s/%s", dastDir, file)
		err = ioutil.WriteFile(destinationFile, dastOpenAPIFile, 0666)
		if err != nil {
			return "", "", errors.New(containerWriteFolderError)
		}
	}

	volumeMap = fmt.Sprintf(containerVolumeFormat, dastDir)
	return volumeMap, dastDir, nil
}

func runDastScan(cmd *cobra.Command, volumeMap, tempDir string, scanType dastScanType, dastWrapper wrappers.DastResultsWrapper) error {
	dastFilePath, _ := cmd.Flags().GetString(commonParams.DastRealTimeFile)
	dastOpenAPIFilePath, _ := cmd.Flags().GetString(commonParams.DastOpenAPIFile)
	_, file := filepath.Split(dastFilePath)
	_, Ofile := filepath.Split(dastOpenAPIFilePath)

	dastRunArgs := []string{
		containerRun,
		containerRemove,
		containerVolumeFlag,
		volumeMap,
		containerNameFlag,
		viper.GetString(commonParams.DastContainerNameKey),
		containerImage,
	}

	dastWebArgs := []string{
		containerWebScan,
	}

	dastAPIArgs := []string{
		containerAPIScan,
		containerOpenAPIFile,
		fmt.Sprintf("%s/%s", containerScanPath, Ofile),
	}

	finalArgs := []string{
		containerPathFile,
		fmt.Sprintf("%s/%s", containerScanPath, file),
		containerOutput,
		containerScanPath,
		containerTimeout,
		"300",
		containerUpdate,
		"10",
		containerJVMProperty,
		"-Xmx512m",
		containerLogLevel,
		"debug",
	}

	var args []string
	if scanType == dastWebScan {
		args = append(dastRunArgs, dastWebArgs...)
	} else {
		args = append(dastRunArgs, dastAPIArgs...)
	}
	args = append(args, finalArgs...)

	output, _ := cmd.Flags().GetString(commonParams.DastOutputFile)

	logger.PrintIfVerbose(containerStarting)
	logger.PrintIfVerbose(containerFormatInfo)
	dastCmd, _ := cmd.Flags().GetString(commonParams.DastRealTimeEngine)

	c := exec.Command(dastCmd, args...)
	if viper.GetBool(commonParams.DebugFlag) {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	} else {
		c.Stdout = io.Discard
		c.Stderr = io.Discard
	}
	err := c.Run()
	if err != nil {
		errorMessage := err.Error()
		// check error code of exec
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() != 0 {
				return errors.Errorf("Check container engine state. Failed: %s", errorMessage)
			}
		}
		if strings.Contains(errorMessage, invalidEngineError) {
			logger.PrintIfVerbose(errorMessage)
			return errors.Errorf(invalidEngineMessage)
		}
		var logs []byte
		logs, err = ioutil.ReadFile(filepath.Join(tempDir, "zap"))
		if err != nil {
			logger.PrintIfVerbose("No zap logs available")
		}

		err = ioutil.WriteFile(filepath.Join(output, "zap.log"), logs, 0666)
		if err != nil {
			logger.PrintIfVerbose("Failed to write zap logs")
		}
	}
	scanTypeString := ""
	switch scanType {
	case dastWebScan:
		scanTypeString = "DAST"
	case dastAPIScan:
		scanTypeString = "DASAPI"
	}

	// there was no error time to handle results
	var resultSummary *wrappers.DastRiskSummary
	resultSummary, err = dastReadResults(output, tempDir, dastWrapper, scanTypeString)
	if err != nil {
		return errors.Errorf("%s", err)
	}
	var resultSummaryJSON []byte
	resultSummaryJSON, err = json.Marshal(resultSummary)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	fmt.Println(string(resultSummaryJSON))

	return nil
}

func dastReadResults(output, dir string, dastWrapper wrappers.DastResultsWrapper, scanType string) (*wrappers.DastRiskSummary, error) {
	// copy logs and results to output folder
	// read results from output folder
	results, err := ioutil.ReadFile(filepath.Join(dir, "ZAP-Report.json"))
	if err != nil {
		return nil, errors.Wrapf(err, "%s", failedToSendResults)
	}

	err = ioutil.WriteFile(filepath.Join(output, "ZAP-Report.json"), results, 0666)
	if err != nil {
		return nil, errors.Wrapf(err, "%s", failedToSendResults)
	}

	// read log from output folder
	log, err := ioutil.ReadFile(filepath.Join(dir, "zap"))
	if err != nil {
		logger.PrintIfVerbose("Failed to get ZAP logs")
	}

	err = ioutil.WriteFile(filepath.Join(output, "zap.log"), log, 0666)
	if err != nil {
		return nil, errors.Wrapf(err, "%s", failedToSendResults)
	}

	// gzip the results
	var b bytes.Buffer
	gw := gzip.NewWriter(&b)
	gw.Write(results)
	gw.Close()

	bodyStruct := struct {
		TenantID   string `json:"tenant_id"`
		ScanID     string `json:"scan_id"`
		Runtime    string `json:"runtime"`
		ResultType string `json:"result_type"`
		Results    []byte `json:"results"`
		Completed  bool   `json:"completed"`
	}{
		ScanID:     uuid.New().String(),
		Runtime:    "ast-cli",
		TenantID:   uuid.New().String(), // should be removed since tenant_id comes from auth token
		ResultType: scanType,
		Results:    b.Bytes(),
		Completed:  true,
	}

	// marshal the body struct to json
	body, err := json.Marshal(bodyStruct)
	if err != nil {
		return nil, errors.Wrapf(err, "%s", failedToSendResults)
	}

	riskLevel, errorModel, err := dastWrapper.SendResults(bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrapf(err, "%s", failedToSendResults)
	}
	if errorModel != nil {
		return nil, errors.Errorf("%s: CODE: %d, %s", failedToSendResults, errorModel.Code, errorModel.Message)
	}

	var total int32
	total += riskLevel.HighCount
	total += riskLevel.MediumCount
	total += riskLevel.LowCount
	total += riskLevel.InfoCount

	return &wrappers.DastRiskSummary{
		SeverityCounter: riskLevel,
		TotalCount:      total,
	}, nil
}
