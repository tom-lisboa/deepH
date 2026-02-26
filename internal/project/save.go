package project

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func SaveRootConfig(workspace string, root RootConfig) error {
	if root.Version <= 0 {
		root.Version = 1
	}
	b, err := yaml.Marshal(&root)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", RootConfigFile, err)
	}
	rootPath := filepath.Join(workspace, RootConfigFile)
	if err := os.WriteFile(rootPath, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rootPath, err)
	}
	return nil
}
