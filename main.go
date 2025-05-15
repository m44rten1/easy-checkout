package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/ktr0731/go-fuzzyfinder"
)

type Branch struct {
	Name      string
	LastUsage time.Time
}

func getGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getBranches() ([]Branch, error) {
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
	branches, err := getBranches()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	idx, err := fuzzyfinder.Find(
		branches,
		func(i int) string {
			branch := branches[i]
			lastUsed := ""
			if !branch.LastUsage.IsZero() {
				lastUsed = fmt.Sprintf(" (%s)", branch.LastUsage.Format("2006-01-02 15:04:05"))
			}
			return branch.Name + lastUsed
		})

	if err != nil {
		if err == fuzzyfinder.ErrAbort {
			os.Exit(0)
		}
		log.Fatal(err)
	}

	selectedBranch := branches[idx].Name
	cmd := exec.Command("git", "checkout", selectedBranch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Error checking out branch: %v", err)
	}
}
