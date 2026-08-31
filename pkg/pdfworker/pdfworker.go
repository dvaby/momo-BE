package pdfworker

import (
	"bytes"
	"fmt"
	"os/exec"
)

func ExtractText(filePath string) (string, error) {
	cmd := exec.Command("pdftotext", "-layout", filePath, "-")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("gagal menjalankan pdftotext: %v, detail: %s", err, stderr.String())
	}

	return stdout.String(), nil
}