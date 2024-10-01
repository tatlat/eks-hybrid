package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// patchEksServiceModel downloads a patch from an S3 bucket to a local directory and applies to the AWS CLI installation
func patchEksServiceModel(awsPatchLocation string, localDir string) error {
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	patchFilePath := filepath.Join(localDir, filepath.Base(awsPatchLocation))

	cmd := exec.Command("aws", "s3", "cp", awsPatchLocation, patchFilePath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to download patch from S3: %v\nOutput: %s", err, string(output))
	}

	fmt.Printf("Successfully downloaded patch to: %s\n", patchFilePath)

	cmd = exec.Command("aws", "configure", "add-model",
		"--service-model", fmt.Sprintf("file://%s", patchFilePath),
		"--service-name", "eksbeta")
	// Run the command and capture output
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply patch: %v\nOutput: %s", err, string(output))
	}

	fmt.Printf("Successfully applied patch to AWS CLI: %s\n", patchFilePath)
	return nil
}
