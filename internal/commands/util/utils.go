package util

import (
	"fmt"
	"os"
	"regexp"

	"github.com/MakeNowJust/heredoc"
	"github.com/checkmarx/ast-cli/internal/commands/util/usercount"
	"github.com/checkmarx/ast-cli/internal/wrappers"
	"github.com/spf13/cobra"
)

const gitURLRegex = "(?P<G1>:git|ssh|https?|git@[-\\w.]+):(\\/\\/)?(?P<G2>.*?)(\\.git)?$"
const sshURLRegex = "^(?P<user>.*?)@(?P<host>.*?):(?:(?P<port>.*?)/)?(?P<path>.*?/.*?)$"

func NewUtilsCommand(gitHubWrapper wrappers.GitHubWrapper,
	azureWrapper wrappers.AzureWrapper,
	bitBucketWrapper wrappers.BitBucketWrapper,
	gitLabWrapper wrappers.GitLabWrapper,
	learnMoreWrapper wrappers.LearnMoreWrapper) *cobra.Command {
	utilsCmd := &cobra.Command{
		Use:   "utils",
		Short: "Utility functions",
		Long:  "The utils command enables the ability to perform CxAST utility functions.",
		Example: heredoc.Doc(
			`
			$ cx utils env
		`,
		),
		Annotations: map[string]string{
			"command:doc": heredoc.Doc(
				`
				https://checkmarx.atlassian.net/wiki/x/VJGXtw
			`,
			),
		},
	}
	envCheckCmd := NewEnvCheckCommand()

	completionCmd := NewCompletionCommand()

	remediationCmd := NewRemediationCommand()

	learnMoreCmd := NewLearnMoreCommand(learnMoreWrapper)

	utilsCmd.AddCommand(completionCmd, envCheckCmd, learnMoreCmd, usercount.NewUserCountCommand(gitHubWrapper, azureWrapper, bitBucketWrapper, gitLabWrapper), remediationCmd)

	return utilsCmd
}

// Contains Tests if a string exists in the provided array/**
func Contains(array []string, val string) bool {
	for _, e := range array {
		if e == val {
			return true
		}
	}
	return false
}

func executeTestCommand(cmd *cobra.Command, args ...string) error {
	fmt.Println("Executing command with args ", args)
	cmd.SetArgs(args)
	cmd.SilenceUsage = false
	return cmd.Execute()
}

// IsGitURL Check if provided URL is a valid git URL (http or ssh)
func IsGitURL(url string) bool {
	compiledRegex := regexp.MustCompile(gitURLRegex)
	urlParts := compiledRegex.FindStringSubmatch(url)

	if urlParts == nil || len(urlParts) < 4 {
		return false
	}

	return len(urlParts[1]) > 0 && len(urlParts[3]) > 0
}

// IsSSHURL Check if provided URL is a valid ssh URL
func IsSSHURL(url string) bool {
	isGitURL, _ := regexp.MatchString(sshURLRegex, url)

	return isGitURL
}

// ReadFileAsString Read a file and return its content as string
func ReadFileAsString(path string) (string, error) {
	content, err := os.ReadFile(path)

	if err != nil {
		return "", err
	}

	return string(content), nil
}
