package scenario

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func LookupJSONPath(raw []byte, path string) (any, bool, error) {
	if path == "" || path == "$" {
		var root any
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, false, err
		}
		return root, true, nil
	}
	if !strings.HasPrefix(path, "$.") {
		return nil, false, fmt.Errorf("only simple $.field paths are supported")
	}

	var current any
	if err := json.Unmarshal(raw, &current); err != nil {
		return nil, false, err
	}

	segments := strings.Split(strings.TrimPrefix(path, "$."), ".")
	for _, segment := range segments {
		if segment == "" {
			return nil, false, fmt.Errorf("invalid empty path segment")
		}
		if segment == "length" {
			list, ok := current.([]any)
			if !ok {
				return nil, false, nil
			}
			current = float64(len(list))
			continue
		}

		name, indexes, err := parseSegment(segment)
		if err != nil {
			return nil, false, err
		}
		if name != "" {
			obj, ok := current.(map[string]any)
			if !ok {
				return nil, false, nil
			}
			next, ok := obj[name]
			if !ok {
				return nil, false, nil
			}
			current = next
		}
		for _, index := range indexes {
			list, ok := current.([]any)
			if !ok || index < 0 || index >= len(list) {
				return nil, false, nil
			}
			current = list[index]
		}
	}

	return current, true, nil
}

func parseSegment(segment string) (string, []int, error) {
	name := segment
	var indexes []int
	if idx := strings.Index(segment, "["); idx >= 0 {
		name = segment[:idx]
		rest := segment[idx:]
		for rest != "" {
			if !strings.HasPrefix(rest, "[") {
				return "", nil, fmt.Errorf("invalid index segment %q", segment)
			}
			end := strings.Index(rest, "]")
			if end < 0 {
				return "", nil, fmt.Errorf("unterminated index segment %q", segment)
			}
			n, err := strconv.Atoi(rest[1:end])
			if err != nil {
				return "", nil, fmt.Errorf("invalid array index %q", rest[1:end])
			}
			indexes = append(indexes, n)
			rest = rest[end+1:]
		}
	}
	return name, indexes, nil
}
