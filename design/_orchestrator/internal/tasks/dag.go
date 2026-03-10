package tasks

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	singleTaskDepRE = regexp.MustCompile(`(?i)^TASK0*(\d+)$`)
	taskRangeDepRE  = regexp.MustCompile(`(?i)^TASK0*(\d+)\s*[–-]\s*(?:TASK)?0*(\d+)$`)
)

// Sort returns entries in a valid execution order.
// Returns an error if a cycle is detected.
func Sort(entries []Entry) ([]Entry, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	taskIndex := make(map[int]int, len(entries))
	taskLabels := make([]string, len(entries))

	for i, entry := range entries {
		number, err := parseTaskNumber(entry.Number)
		if err != nil {
			return nil, fmt.Errorf("task %q: %w", entry.Number, err)
		}
		if existing, ok := taskIndex[number]; ok {
			return nil, fmt.Errorf(
				"duplicate task number %s at entries %d and %d",
				formatTaskLabel(entries[existing].Number),
				existing+1,
				i+1,
			)
		}
		taskIndex[number] = i
		taskLabels[i] = formatTaskLabel(entry.Number)
	}

	adjacency := make([][]int, len(entries))
	indegree := make([]int, len(entries))

	for i, entry := range entries {
		seenDeps := make(map[int]struct{})

		for _, dep := range entry.DependsOn {
			dependencyIndexes, enforced, err := resolveDependencyIndexes(dep, taskIndex)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", taskLabels[i], err)
			}
			if !enforced {
				continue
			}

			for _, dependencyIndex := range dependencyIndexes {
				if _, ok := seenDeps[dependencyIndex]; ok {
					continue
				}
				seenDeps[dependencyIndex] = struct{}{}
				adjacency[dependencyIndex] = append(adjacency[dependencyIndex], i)
				indegree[i]++
			}
		}
	}

	queue := make([]int, 0, len(entries))
	for i := range entries {
		if indegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	ordered := make([]Entry, 0, len(entries))
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		ordered = append(ordered, entries[current])

		for _, next := range adjacency[current] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(ordered) == len(entries) {
		return ordered, nil
	}

	var blocked []string
	for i := range entries {
		if indegree[i] > 0 {
			blocked = append(blocked, taskLabels[i])
		}
	}
	sort.Strings(blocked)

	return nil, fmt.Errorf("dependency cycle detected among %s", strings.Join(blocked, ", "))
}

func resolveDependencyIndexes(dep string, taskIndex map[int]int) ([]int, bool, error) {
	dep = strings.TrimSpace(dep)
	if dep == "" || dep == "—" {
		return nil, false, nil
	}

	if matches := singleTaskDepRE.FindStringSubmatch(dep); matches != nil {
		number, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, false, fmt.Errorf("invalid dependency %q", dep)
		}
		index, ok := taskIndex[number]
		if !ok {
			return nil, true, fmt.Errorf("depends on missing task %s", normalizeTaskDependency(dep))
		}
		return []int{index}, true, nil
	}

	if matches := taskRangeDepRE.FindStringSubmatch(dep); matches != nil {
		start, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, false, fmt.Errorf("invalid dependency range %q", dep)
		}
		end, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, false, fmt.Errorf("invalid dependency range %q", dep)
		}
		if end < start {
			return nil, true, fmt.Errorf("invalid dependency range %q", dep)
		}

		indexes := make([]int, 0, end-start+1)
		for number := start; number <= end; number++ {
			index, ok := taskIndex[number]
			if !ok {
				continue
			}
			indexes = append(indexes, index)
		}
		if len(indexes) == 0 {
			return nil, true, fmt.Errorf("depends on missing task range %s", dep)
		}
		return indexes, true, nil
	}

	// Cross-version references are preserved by Parse but intentionally not
	// enforced by the v1 DAG.
	return nil, false, nil
}

func parseTaskNumber(value string) (int, error) {
	value = strings.TrimSpace(value)
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid task number %q", value)
	}
	if number <= 0 {
		return 0, fmt.Errorf("invalid task number %q", value)
	}
	return number, nil
}

func formatTaskLabel(number string) string {
	number = strings.TrimSpace(number)
	if parsed, err := strconv.Atoi(number); err == nil {
		return fmt.Sprintf("TASK%02d", parsed)
	}
	return "TASK" + number
}

func normalizeTaskDependency(dep string) string {
	if matches := singleTaskDepRE.FindStringSubmatch(strings.TrimSpace(dep)); matches != nil {
		if number, err := strconv.Atoi(matches[1]); err == nil {
			return fmt.Sprintf("TASK%02d", number)
		}
	}
	return dep
}
