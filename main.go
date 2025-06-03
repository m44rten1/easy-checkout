package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"easy-checkout/version"

	"github.com/ktr0731/go-fuzzyfinder"
)

type Branch struct {
	Name      string
	LastUsage time.Time
	IsCurrent bool
}

func getGitCommand(args ...string) (string, error) {
	// First check if we're in a git repository
	if args[0] != "rev-parse" { // Skip this check for rev-parse itself
		checkCmd := exec.Command("git", "rev-parse", "--git-dir")
		if err := checkCmd.Run(); err != nil {
			return "", fmt.Errorf("not in a git repository - please run this command from within a git repository")
		}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getCurrentBranch() (string, error) {
	output, err := getGitCommand("branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("error getting current branch: %v", err)
	}
	return output, nil
}

func getBranches() ([]Branch, error) {
	// Get current branch first
	currentBranch, err := getCurrentBranch()
	if err != nil {
		return nil, err
	}

	// Get all local branches
	branchOutput, err := getGitCommand("branch")
	if err != nil {
		return nil, fmt.Errorf("error getting branches: %v", err)
	}

	// Get all remote branches
	remoteBranchOutput, err := getGitCommand("branch", "-r")
	if err != nil {
		return nil, fmt.Errorf("error getting remote branches: %v", err)
	}

	branchLines := strings.Split(branchOutput, "\n")
	remoteBranchLines := strings.Split(remoteBranchOutput, "\n")
	branches := make(map[string]Branch)

	// Initialize local branches with names
	for _, line := range branchLines {
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if name == "" {
			continue
		}
		branches[name] = Branch{
			Name:      name,
			LastUsage: time.Time{}, // Zero time as default
			IsCurrent: name == currentBranch,
		}
	}

	// Get list of local branch names for filtering remotes
	localBranchNames := make(map[string]bool)
	for name := range branches {
		localBranchNames[name] = true
	}

	// Initialize remote branches with names
	for _, line := range remoteBranchLines {
		name := strings.TrimSpace(line)
		if name == "" || strings.Contains(name, "HEAD ->") {
			continue
		}
		// Extract branch name without remote prefix
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			continue
		}
		branchName := parts[1]

		// Skip if we already have this branch locally
		if localBranchNames[branchName] {
			continue
		}

		branches[name] = Branch{
			Name:      name,
			LastUsage: time.Time{}, // Zero time as default
			IsCurrent: false,       // Remote branches can't be current
		}
	}

	// Get reflog entries with ISO dates
	reflogOutput, err := getGitCommand("reflog", "show", "--date=iso")
	if err != nil {
		return nil, fmt.Errorf("error getting reflog: %v", err)
	}

	reflogLines := strings.Split(reflogOutput, "\n")
	for _, line := range reflogLines {
		if !strings.Contains(line, "checkout: moving") {
			continue
		}

		// Extract timestamp
		parts := strings.SplitN(line, "HEAD@{", 2)
		if len(parts) != 2 {
			continue
		}
		timestampPart := strings.Split(parts[1], "}")
		if len(timestampPart) < 2 {
			continue
		}

		// Parse the ISO format timestamp
		timestamp, err := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(timestampPart[0]))
		if err != nil {
			continue
		}

		// Extract branch name - it's the last part after "to "
		if idx := strings.LastIndex(line, "to "); idx != -1 {
			branchName := strings.TrimSpace(line[idx+3:])
			if branch, exists := branches[branchName]; exists {
				if branch.LastUsage.IsZero() || timestamp.After(branch.LastUsage) {
					branch.LastUsage = timestamp
					branches[branchName] = branch
				}
			}
		}
	}

	// Convert map to slice and sort by last usage
	var branchList []Branch
	for _, branch := range branches {
		branchList = append(branchList, branch)
	}

	// Sort branches by last usage time, most recent first
	sort.Slice(branchList, func(i, j int) bool {
		// If one branch has never been used, put it last
		if branchList[i].LastUsage.IsZero() {
			return false
		}
		if branchList[j].LastUsage.IsZero() {
			return true
		}
		return branchList[i].LastUsage.After(branchList[j].LastUsage)
	})

	return branchList, nil
}

func checkGitVersion() error {
	output, err := exec.Command("git", "--version").Output()
	if err != nil {
		return fmt.Errorf("failed to get git version: %v", err)
	}

	// Use regex to extract version number (e.g., 2.39.5)
	versionStr := strings.TrimSpace(string(output))
	versionRegex := "\\d+\\.\\d+(?:\\.\\d+)?"
	re := regexp.MustCompile(versionRegex)
	versionMatch := re.FindString(versionStr)
	if versionMatch == "" {
		return fmt.Errorf("could not parse git version from: %s", versionStr)
	}

	versionParts := strings.Split(versionMatch, ".")
	if len(versionParts) < 2 {
		return fmt.Errorf("unexpected git version format: %s", versionMatch)
	}

	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return fmt.Errorf("failed to parse git major version: %v", err)
	}

	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return fmt.Errorf("failed to parse git minor version: %v", err)
	}

	if major < 2 || (major == 2 && minor < 22) {
		return fmt.Errorf("git version %s is too old. Please upgrade to git 2.22.0 or later", versionMatch)
	}

	return nil
}

func main() {
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s\n", version.Version)
		os.Exit(0)
	}

	if err := checkGitVersion(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	branches, err := getBranches()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	idx, err := fuzzyfinder.Find(
		branches,
		func(i int) string {
			branch := branches[i]
			prefix := "  "
			if branch.IsCurrent {
				prefix = "* "
			}

			// Handle remote branches differently
			if strings.Contains(branch.Name, "/") {
				return fmt.Sprintf("%s%s%s", prefix, strings.Repeat(" ", 18), branch.Name)
			}

			// Local branches with timestamp
			timestamp := "                    " // 20 spaces for alignment
			if !branch.LastUsage.IsZero() {
				timestamp = branch.LastUsage.Format("02/01/06 15:04")
			}
			return fmt.Sprintf("%s%s    %s", prefix, timestamp, branch.Name)
		})

	if err != nil {
		if err == fuzzyfinder.ErrAbort {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	selectedBranch := branches[idx].Name

	// Check if this is a remote branch
	if strings.Contains(selectedBranch, "/") {
		// Extract the branch name without remote prefix
		parts := strings.SplitN(selectedBranch, "/", 2)
		if len(parts) == 2 {
			localBranch := parts[1]

			// Check if the local branch already exists
			if _, err := getGitCommand("rev-parse", "--verify", localBranch); err == nil {
				// Branch exists, just check it out
				cmd := exec.Command("git", "checkout", localBranch)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Error checking out existing branch: %v\n", err)
					os.Exit(1)
				}
				return
			}

			// Branch doesn't exist, create a new local branch tracking the remote branch
			cmd := exec.Command("git", "checkout", "-b", localBranch, selectedBranch)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating tracking branch: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// For local branches, proceed as normal
	cmd := exec.Command("git", "checkout", selectedBranch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error checking out branch: %v\n", err)
		os.Exit(1)
	}
}
