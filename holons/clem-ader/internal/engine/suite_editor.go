package engine

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type suiteEditor struct {
	path string
	doc  yaml.Node
}

func loadSuiteEditor(path string) (*suiteEditor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("suite file %s is not a YAML document", path)
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("suite file %s root must be a mapping", path)
	}
	return &suiteEditor{path: path, doc: doc}, nil
}

func (e *suiteEditor) stepLane(stepID string) (string, bool, error) {
	stepNode, err := e.stepNode(stepID)
	if err != nil {
		return "", false, err
	}
	if laneNode, ok := mappingValue(stepNode, "lane"); ok {
		if laneNode.Kind != yaml.ScalarNode {
			return "", false, fmt.Errorf("suite file %s step %q field %q must be a scalar", e.path, stepID, "lane")
		}
		return normalizeStepLane(laneNode.Value), true, nil
	}
	return "progression", false, nil
}

func (e *suiteEditor) setStepLane(stepID string, lane string) error {
	lane = normalizeStepLane(lane)
	if lane != "progression" && lane != "regression" {
		return fmt.Errorf("unsupported lane %q", lane)
	}
	stepNode, err := e.stepNode(stepID)
	if err != nil {
		return err
	}
	if laneNode, ok := mappingValue(stepNode, "lane"); ok {
		laneNode.Kind = yaml.ScalarNode
		laneNode.Tag = "!!str"
		laneNode.Value = lane
		return nil
	}
	stepNode.Content = append(stepNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "lane"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: lane},
	)
	return nil
}

func (e *suiteEditor) stepNode(stepID string) (*yaml.Node, error) {
	root := e.doc.Content[0]
	stepsNode, ok := mappingValue(root, "steps")
	if !ok || stepsNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("suite file %s does not define steps", e.path)
	}
	stepNode, ok := mappingValue(stepsNode, stepID)
	if !ok || stepNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("suite file %s does not define step %q", e.path, stepID)
	}
	return stepNode, nil
}

func (e *suiteEditor) write() error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&e.doc); err != nil {
		_ = encoder.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return os.WriteFile(e.path, buf.Bytes(), 0o644)
}

func mappingValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, false
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if strings.TrimSpace(node.Content[index].Value) == key {
			return node.Content[index+1], true
		}
	}
	return nil, false
}

func orderedUniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
