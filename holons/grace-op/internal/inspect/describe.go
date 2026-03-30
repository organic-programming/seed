package inspect

import (
	"strings"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
)

// FromDescribeResponse normalizes a HolonMeta.Describe response into the
// offline inspection document shape used by op inspect.
func FromDescribeResponse(response *holonsv1.DescribeResponse) *Document {
	if response == nil {
		return &Document{}
	}

	services := make([]Service, 0, len(response.GetServices()))
	for _, service := range response.GetServices() {
		methods := make([]Method, 0, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			methods = append(methods, Method{
				Name:            method.GetName(),
				Description:     method.GetDescription(),
				InputType:       method.GetInputType(),
				OutputType:      method.GetOutputType(),
				InputFields:     fromDescribeFields(method.GetInputFields()),
				OutputFields:    fromDescribeFields(method.GetOutputFields()),
				ClientStreaming: method.GetClientStreaming(),
				ServerStreaming: method.GetServerStreaming(),
				ExampleInput:    method.GetExampleInput(),
			})
		}

		services = append(services, Service{
			Name:        service.GetName(),
			Description: service.GetDescription(),
			Methods:     methods,
		})
	}

	manifest := response.GetManifest()
	identity := manifest.GetIdentity()
	skills := make([]Skill, 0, len(manifest.GetSkills()))
	for _, skill := range manifest.GetSkills() {
		skills = append(skills, Skill{
			Name:        skill.GetName(),
			Description: skill.GetDescription(),
			When:        skill.GetWhen(),
			Steps:       append([]string(nil), skill.GetSteps()...),
		})
	}

	sequences := make([]Sequence, 0, len(manifest.GetSequences()))
	for _, sequence := range manifest.GetSequences() {
		params := make([]SequenceParam, 0, len(sequence.GetParams()))
		for _, param := range sequence.GetParams() {
			params = append(params, SequenceParam{
				Name:        param.GetName(),
				Description: param.GetDescription(),
				Required:    param.GetRequired(),
				Default:     param.GetDefault(),
			})
		}
		sequences = append(sequences, Sequence{
			Name:        sequence.GetName(),
			Description: sequence.GetDescription(),
			Params:      params,
			Steps:       append([]string(nil), sequence.GetSteps()...),
		})
	}

	return &Document{
		Slug:      slugFromIdentity(identity.GetGivenName(), identity.GetFamilyName()),
		Motto:     identity.GetMotto(),
		Services:  services,
		Skills:    skills,
		Sequences: sequences,
	}
}

func fromDescribeFields(fields []*holonsv1.FieldDoc) []Field {
	out := make([]Field, 0, len(fields))
	for _, field := range fields {
		out = append(out, Field{
			Name:         field.GetName(),
			Type:         field.GetType(),
			Number:       field.GetNumber(),
			Description:  field.GetDescription(),
			Label:        describeFieldLabel(field.GetLabel()),
			MapKeyType:   field.GetMapKeyType(),
			MapValueType: field.GetMapValueType(),
			NestedFields: fromDescribeFields(field.GetNestedFields()),
			EnumValues:   fromDescribeEnumValues(field.GetEnumValues()),
			Required:     field.GetRequired(),
			Example:      field.GetExample(),
		})
	}
	return out
}

func fromDescribeEnumValues(values []*holonsv1.EnumValueDoc) []EnumValue {
	out := make([]EnumValue, 0, len(values))
	for _, value := range values {
		out = append(out, EnumValue{
			Name:        value.GetName(),
			Number:      value.GetNumber(),
			Description: value.GetDescription(),
		})
	}
	return out
}

func describeFieldLabel(label holonsv1.FieldLabel) string {
	switch label {
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return FieldLabelRepeated
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return FieldLabelMap
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return FieldLabelRequired
	default:
		return FieldLabelOptional
	}
}

func slugFromIdentity(givenName, familyName string) string {
	given := strings.TrimSpace(givenName)
	family := strings.TrimSpace(strings.TrimSuffix(familyName, "?"))
	if given == "" && family == "" {
		return ""
	}
	return strings.Trim(strings.ToLower(strings.ReplaceAll(given+"-"+family, " ", "-")), "-")
}
