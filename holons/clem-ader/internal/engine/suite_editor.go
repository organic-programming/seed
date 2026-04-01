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

func (e *suiteEditor) downgradeProfile(profile string, stepIDs []string, all bool) ([]string, []string, error) {
	regressionNode, progressionNode, err := e.profileLaneNodes(profile)
	if err != nil {
		return nil, nil, err
	}
	regression, err := sequenceValues(regressionNode)
	if err != nil {
		return nil, nil, err
	}
	progression, err := sequenceValues(progressionNode)
	if err != nil {
		return nil, nil, err
	}

	var moved []string
	var ignored []string
	if all {
		moved = append(moved, regression...)
	} else {
		targets := orderedUniqueStrings(stepIDs)
		for _, id := range regression {
			if containsString(targets, id) {
				moved = append(moved, id)
			}
		}
		for _, id := range targets {
			if !containsString(moved, id) {
				ignored = append(ignored, id)
			}
		}
	}

	if len(moved) == 0 {
		return nil, ignored, nil
	}

	newRegression := make([]string, 0, len(regression))
	for _, id := range regression {
		if !containsString(moved, id) {
			newRegression = append(newRegression, id)
		}
	}
	newProgression := append([]string(nil), progression...)
	for _, id := range moved {
		if !containsString(newProgression, id) {
			newProgression = append(newProgression, id)
		}
	}

	setSequenceValues(regressionNode, newRegression)
	setSequenceValues(progressionNode, newProgression)
	return moved, ignored, nil
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

func (e *suiteEditor) profileLaneNodes(profile string) (*yaml.Node, *yaml.Node, error) {
	root := e.doc.Content[0]
	profilesNode, ok := mappingValue(root, "profiles")
	if !ok || profilesNode.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("suite file %s does not define profiles", e.path)
	}
	profileNode, ok := mappingValue(profilesNode, profile)
	if !ok || profileNode.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("suite file %s does not define profile %q", e.path, profile)
	}
	regressionNode, err := ensureSequenceValue(profileNode, "regression")
	if err != nil {
		return nil, nil, err
	}
	progressionNode, err := ensureSequenceValue(profileNode, "progression")
	if err != nil {
		return nil, nil, err
	}
	return regressionNode, progressionNode, nil
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

func ensureSequenceValue(mapping *yaml.Node, key string) (*yaml.Node, error) {
	if value, ok := mappingValue(mapping, key); ok {
		if value.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("profile lane %q must be a sequence", key)
		}
		return value, nil
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
	return valueNode, nil
}

func sequenceValues(node *yaml.Node) ([]string, error) {
	if node == nil {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("expected YAML sequence node")
	}
	values := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("expected YAML scalar sequence item")
		}
		value := strings.TrimSpace(item.Value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values, nil
}

func setSequenceValues(node *yaml.Node, values []string) {
	node.Kind = yaml.SequenceNode
	node.Tag = "!!seq"
	node.Content = node.Content[:0]
	for _, value := range values {
		node.Content = append(node.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: value,
		})
	}
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
