package cmd

import (
	"example/sensorHub/skills"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:     "skills",
	GroupID: hubGroupID,
	Short:   "Manage LLM skill files for AI assistant integration",
}

var skillsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the skill file content to stdout",
	RunE:  runSkillsShow,
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install skill files for LLM assistants",
	Long:  "Install skill files to LLM assistant directories.\nUse --target to specify which assistant, or --all to install for all detected assistants.",
	RunE:  runSkillsInstall,
}

var skillTarget string
var skillAll bool

var skillShowTarget string

func init() {
	skillsShowCmd.Flags().StringVar(&skillShowTarget, "target", "copilot", "Target LLM assistant (copilot, claude)")
	skillsInstallCmd.Flags().StringVar(&skillTarget, "target", "", "Target LLM assistant (copilot, claude)")
	skillsInstallCmd.Flags().BoolVar(&skillAll, "all", false, "Install for all detected LLM assistants")

	skillsCmd.AddCommand(skillsShowCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	rootCmd.AddCommand(skillsCmd)
}

type skillTarget_ struct {
	name     string
	file     string
	dirPath  string
	filePath string
}

func getSkillTargets() []skillTarget_ {
	home, _ := os.UserHomeDir()
	return []skillTarget_{
		{
			name:     "copilot",
			file:     "copilot.md",
			dirPath:  filepath.Join(home, ".copilot", "skills", "sensor-hub"),
			filePath: filepath.Join(home, ".copilot", "skills", "sensor-hub", "SKILL.md"),
		},
		{
			name:     "claude",
			file:     "claude.md",
			dirPath:  filepath.Join(home, ".claude", "skills", "sensor-hub"),
			filePath: filepath.Join(home, ".claude", "skills", "sensor-hub", "SKILL.md"),
		},
	}
}

func runSkillsShow(cmd *cobra.Command, args []string) error {
	fileName := skillShowTarget + ".md"
	content, err := skills.SkillFiles.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("unknown target: %s (valid: copilot, claude)", skillShowTarget)
	}
	fmt.Print(string(content))
	return nil
}

func runSkillsInstall(cmd *cobra.Command, args []string) error {
	if skillTarget == "" && !skillAll {
		return fmt.Errorf("specify --target (copilot, claude) or --all")
	}

	targets := getSkillTargets()
	installed := 0

	for _, t := range targets {
		if !skillAll && t.name != skillTarget {
			continue
		}

		content, err := skills.SkillFiles.ReadFile(t.file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠ Could not read skill file for %s: %v\n", t.name, err)
			continue
		}

		if skillAll {
			// Only install if the parent directory exists
			parentDir := filepath.Dir(filepath.Dir(t.dirPath))
			if _, err := os.Stat(parentDir); os.IsNotExist(err) {
				fmt.Printf("Skipping %s (directory %s not found)\n", t.name, parentDir)
				continue
			}
		}

		if err := os.MkdirAll(t.dirPath, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ Failed to create directory %s: %v\n", t.dirPath, err)
			continue
		}

		if err := os.WriteFile(t.filePath, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ Failed to write skill file to %s: %v\n", t.filePath, err)
			continue
		}

		fmt.Printf("✓ Installed sensor-hub skill to %s\n", t.filePath)
		installed++
	}

	if installed == 0 && skillTarget != "" {
		return fmt.Errorf("unknown target: %s (valid: copilot, claude)", skillTarget)
	}

	return nil
}
