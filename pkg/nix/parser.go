// parser.go
package nix

import (
	"fmt"
	"strconv"
	"strings"
)

// parseNARInfo parses a .narinfo file
func parseNARInfo(content string) (*NARInfo, error) {
	info := &NARInfo{}
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "StorePath":
			info.StorePath = value
		case "URL":
			info.URL = value
		case "Compression":
			info.Compression = value
		case "FileHash":
			info.FileHash = strings.TrimPrefix(value, "sha256:")
		case "FileSize":
			size, _ := strconv.ParseInt(value, 10, 64)
			info.FileSize = size
		case "NarHash":
			info.NarHash = strings.TrimPrefix(value, "sha256:")
		case "NarSize":
			size, _ := strconv.ParseInt(value, 10, 64)
			info.NarSize = size
		case "References":
			if value != "" {
				info.References = strings.Fields(value)
			}
		case "Deriver":
			info.Deriver = value
		case "Sig":
			info.Signature = value
		}
	}

	if info.StorePath == "" {
		return nil, fmt.Errorf("missing StorePath in narinfo")
	}

	return info, nil
}