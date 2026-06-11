package agent

import (
	"fmt"
	"os/exec"
)

func RunGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v failed: %s", args, out)
	}
	return nil
}

func CreateBranch(branchName string) error {
	return RunGitCommand("checkout", "-b", branchName)
}

func AddAndCommit(file, msg string) error {
	RunGitCommand("add", file)
	return RunGitCommand("commit", "-m", msg)
}

func PushBranch(branchName string) error {
	return RunGitCommand("push", "origin", branchName)
}
