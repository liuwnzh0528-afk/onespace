package config

import (
	"path/filepath"
	"strings"

	"github.com/wnzhone/onespace/internal/domain"
)

func pathUnderAnyRoot(path string, roots []string) bool {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		cleanRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(cleanRoot, cleanPath)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			return true
		}
	}
	return false
}

func setServiceName(name string, svc domain.Service) domain.Service {
	svc.Name = name
	return svc
}
