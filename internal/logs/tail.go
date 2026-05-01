package logs

import (
	"os"
	"strings"
)

func tailFile(path string, lines int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return nil, nil
	}

	allLines := strings.Split(content, "\n")
	if lines >= len(allLines) {
		return allLines, nil
	}
	return allLines[len(allLines)-lines:], nil
}
