package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
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

	// Get all branches
	branchOutput, err := getGitCommand("branch")
	if err != nil {
		return nil, fmt.Errorf("error getting branches: %v", err)
	}

	branchLines := strings.Split(branchOutput, "\n")
	branches := make(map[string]Branch)

	// Initialize branches with names
	for _, line := range branchLines {
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		branches[name] = Branch{
			Name:      name,
			LastUsage: time.Time{}, // Zero time as default
			IsCurrent: name == currentBranch,
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

func main() {
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s\n", version.Version)
		os.Exit(0)
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
			timestamp := "                    " // 20 spaces for alignment
			if !branch.LastUsage.IsZero() {
				timestamp = branch.LastUsage.Format("02/01/06 15:04")
			}

			prefix := "  "
			if branch.IsCurrent {
				prefix = "* "
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
	cmd := exec.Command("git", "checkout", selectedBranch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error checking out branch: %v\n", err)
		os.Exit(1)
	}
}
