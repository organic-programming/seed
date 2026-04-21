package inspect

// Document is the normalized output for op inspect, regardless of whether the
// source is offline proto parsing or HolonMeta.Describe.
type Document struct {
	Slug      string     `json:"slug,omitempty"`
	Motto     string     `json:"motto,omitempty"`
	Services  []Service  `json:"services"`
	Skills    []Skill    `json:"skills,omitempty"`
	Sequences []Sequence `json:"sequences,omitempty"`
}

type Service struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Methods     []Method `json:"methods,omitempty"`
}

type Method struct {
	Name            string     `json:"name"`
	Description     string     `json:"description,omitempty"`
	InputType       string     `json:"input_type,omitempty"`
	OutputType      string     `json:"output_type,omitempty"`
	InputFields     []Field    `json:"input_fields,omitempty"`
	OutputFields    []Field    `json:"output_fields,omitempty"`
	ClientStreaming bool       `json:"client_streaming,omitempty"`
	ServerStreaming bool       `json:"server_streaming,omitempty"`
	Examples        [][]string `json:"examples,omitempty"`
}

// ExampleInput returns the first example's first token, or "" if absent.
// Deprecated: use Examples directly.
func (m Method) ExampleInput() string {
	if len(m.Examples) > 0 && len(m.Examples[0]) > 0 {
		return m.Examples[0][0]
	}
	return ""
}

type Field struct {
	Name         string      `json:"name"`
	Type         string      `json:"type,omitempty"`
	Number       int32       `json:"number,omitempty"`
	Description  string      `json:"description,omitempty"`
	Label        string      `json:"label,omitempty"`
	MapKeyType   string      `json:"map_key_type,omitempty"`
	MapValueType string      `json:"map_value_type,omitempty"`
	NestedFields []Field     `json:"nested_fields,omitempty"`
	EnumValues   []EnumValue `json:"enum_values,omitempty"`
	Required     bool        `json:"required,omitempty"`
	Example      string      `json:"example,omitempty"`
}

type EnumValue struct {
	Name        string `json:"name"`
	Number      int32  `json:"number,omitempty"`
	Description string `json:"description,omitempty"`
}

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	When        string   `json:"when,omitempty"`
	Steps       []string `json:"steps,omitempty"`
}

type Sequence struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Params      []SequenceParam `json:"params,omitempty"`
	Steps       []string        `json:"steps,omitempty"`
}

type SequenceParam struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

const (
	FieldLabelOptional = "optional"
	FieldLabelRepeated = "repeated"
	FieldLabelMap      = "map"
	FieldLabelRequired = "required"
)
