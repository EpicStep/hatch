package knownhosts

import (
	"fmt"
	"os"
	"strings"
)

// RemoveEntry removes all known_hosts entries matching the given host and port.
// For non-standard ports, SSH stores entries as [host]:port.
// Hashed entries are left untouched. No-op if the file does not exist.
func RemoveEntry(path string, host string, port int) error {
	var target string
	if port == 22 {
		target = host
	} else {
		target = fmt.Sprintf("[%s]:%d", host, port)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "|1|") {
			kept = append(kept, line)
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			kept = append(kept, line)
			continue
		}

		hosts := strings.Split(fields[0], ",")
		match := false
		for _, h := range hosts {
			if h == target {
				match = true
				break
			}
		}

		if !match {
			kept = append(kept, line)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), info.Mode())
}
