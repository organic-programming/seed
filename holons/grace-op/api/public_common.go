package api

import (
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	dopkg "github.com/organic-programming/grace-op/internal/do"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
	opmod "github.com/organic-programming/grace-op/internal/mod"
	"github.com/organic-programming/grace-op/internal/scaffold"
)

func identityToProto(id identity.Identity) *opv1.HolonIdentity {
	return &opv1.HolonIdentity{
		Uuid:         id.UUID,
		GivenName:    id.GivenName,
		FamilyName:   id.FamilyName,
		Motto:        id.Motto,
		Composer:     id.Composer,
		Clade:        cladeToProto(id.Clade),
		Status:       statusToProto(id.Status),
		Born:         id.Born,
		Parents:      id.Parents,
		Reproduction: reproductionToProto(id.Reproduction),
		Aliases:      id.Aliases,
		GeneratedBy:  id.GeneratedBy,
		Lang:         id.Lang,
		ProtoStatus:  statusToProto(id.ProtoStatus),
	}
}

func localHolonToProto(h holons.LocalHolon) *opv1.HolonEntry {
	return &opv1.HolonEntry{
		Identity:     identityToProto(h.Identity),
		Origin:       h.Origin,
		RelativePath: h.RelativePath,
	}
}

func cladeToProto(value string) opv1.Clade {
	switch lowerTrim(value) {
	case "deterministic/pure":
		return opv1.Clade_DETERMINISTIC_PURE
	case "deterministic/stateful":
		return opv1.Clade_DETERMINISTIC_STATEFUL
	case "deterministic/io_bound":
		return opv1.Clade_DETERMINISTIC_IO_BOUND
	case "probabilistic/generative":
		return opv1.Clade_PROBABILISTIC_GENERATIVE
	case "probabilistic/perceptual":
		return opv1.Clade_PROBABILISTIC_PERCEPTUAL
	case "probabilistic/adaptive":
		return opv1.Clade_PROBABILISTIC_ADAPTIVE
	default:
		return opv1.Clade_CLADE_UNSPECIFIED
	}
}

func statusToProto(value string) opv1.Status {
	switch lowerTrim(value) {
	case "draft":
		return opv1.Status_DRAFT
	case "stable":
		return opv1.Status_STABLE
	case "deprecated":
		return opv1.Status_DEPRECATED
	case "dead":
		return opv1.Status_DEAD
	default:
		return opv1.Status_STATUS_UNSPECIFIED
	}
}

func reproductionToProto(value string) opv1.ReproductionMode {
	switch lowerTrim(value) {
	case "manual":
		return opv1.ReproductionMode_MANUAL
	case "assisted":
		return opv1.ReproductionMode_ASSISTED
	case "automatic":
		return opv1.ReproductionMode_AUTOMATIC
	case "autopoietic":
		return opv1.ReproductionMode_AUTOPOIETIC
	case "bred":
		return opv1.ReproductionMode_BRED
	default:
		return opv1.ReproductionMode_REPRODUCTION_UNSPECIFIED
	}
}

func lowerTrim(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func buildOptionsFromProto(options *opv1.BuildOptions) holons.BuildOptions {
	if options == nil {
		return holons.BuildOptions{}
	}
	return holons.BuildOptions{
		Target: options.GetTarget(),
		Mode:   options.GetMode(),
		DryRun: options.GetDryRun(),
		NoSign: options.GetNoSign(),
	}
}

func lifecycleReportToProto(report holons.Report) *opv1.LifecycleReport {
	out := &opv1.LifecycleReport{
		Operation:   report.Operation,
		Target:      report.Target,
		Holon:       report.Holon,
		Dir:         report.Dir,
		Manifest:    report.Manifest,
		Runner:      report.Runner,
		Kind:        report.Kind,
		Binary:      report.Binary,
		BuildTarget: report.BuildTarget,
		BuildMode:   report.BuildMode,
		Artifact:    report.Artifact,
		Commands:    append([]string(nil), report.Commands...),
		Notes:       append([]string(nil), report.Notes...),
	}
	for _, child := range report.Children {
		out.Children = append(out.Children, lifecycleReportToProto(child))
	}
	return out
}

func installReportToProto(report holons.InstallReport) *opv1.InstallReport {
	return &opv1.InstallReport{
		Operation:   report.Operation,
		Target:      report.Target,
		Holon:       report.Holon,
		Dir:         report.Dir,
		Manifest:    report.Manifest,
		Binary:      report.Binary,
		BuildTarget: report.BuildTarget,
		BuildMode:   report.BuildMode,
		Artifact:    report.Artifact,
		Installed:   report.Installed,
		Notes:       append([]string(nil), report.Notes...),
	}
}

func inspectDocumentToProto(doc *inspectpkg.Document) *opv1.InspectDocument {
	if doc == nil {
		return nil
	}
	out := &opv1.InspectDocument{
		Slug:  doc.Slug,
		Motto: doc.Motto,
	}
	for _, service := range doc.Services {
		serviceOut := &opv1.InspectService{
			Name:        service.Name,
			Description: service.Description,
		}
		for _, method := range service.Methods {
			methodOut := &opv1.InspectMethod{
				Name:            method.Name,
				Description:     method.Description,
				InputType:       method.InputType,
				OutputType:      method.OutputType,
				ClientStreaming: method.ClientStreaming,
				ServerStreaming: method.ServerStreaming,
				ExampleInput:    method.ExampleInput,
			}
			for _, field := range method.InputFields {
				methodOut.InputFields = append(methodOut.InputFields, inspectFieldToProto(field))
			}
			for _, field := range method.OutputFields {
				methodOut.OutputFields = append(methodOut.OutputFields, inspectFieldToProto(field))
			}
			serviceOut.Methods = append(serviceOut.Methods, methodOut)
		}
		out.Services = append(out.Services, serviceOut)
	}
	for _, skill := range doc.Skills {
		out.Skills = append(out.Skills, &opv1.InspectSkill{
			Name:        skill.Name,
			Description: skill.Description,
			When:        skill.When,
			Steps:       append([]string(nil), skill.Steps...),
		})
	}
	for _, sequence := range doc.Sequences {
		sequenceOut := &opv1.InspectSequence{
			Name:        sequence.Name,
			Description: sequence.Description,
			Steps:       append([]string(nil), sequence.Steps...),
		}
		for _, param := range sequence.Params {
			sequenceOut.Params = append(sequenceOut.Params, &opv1.InspectSequenceParam{
				Name:        param.Name,
				Description: param.Description,
				Required:    param.Required,
				Default:     param.Default,
			})
		}
		out.Sequences = append(out.Sequences, sequenceOut)
	}
	return out
}

func inspectFieldToProto(field inspectpkg.Field) *opv1.InspectField {
	out := &opv1.InspectField{
		Name:         field.Name,
		Type:         field.Type,
		Number:       field.Number,
		Description:  field.Description,
		Label:        field.Label,
		MapKeyType:   field.MapKeyType,
		MapValueType: field.MapValueType,
		Required:     field.Required,
		Example:      field.Example,
	}
	for _, nested := range field.NestedFields {
		out.NestedFields = append(out.NestedFields, inspectFieldToProto(nested))
	}
	for _, enumValue := range field.EnumValues {
		out.EnumValues = append(out.EnumValues, &opv1.InspectEnumValue{
			Name:        enumValue.Name,
			Number:      enumValue.Number,
			Description: enumValue.Description,
		})
	}
	return out
}

func inspectDocumentFromProto(doc *opv1.InspectDocument) *inspectpkg.Document {
	if doc == nil {
		return nil
	}
	out := &inspectpkg.Document{
		Slug:  doc.GetSlug(),
		Motto: doc.GetMotto(),
	}
	for _, service := range doc.GetServices() {
		serviceOut := inspectpkg.Service{
			Name:        service.GetName(),
			Description: service.GetDescription(),
		}
		for _, method := range service.GetMethods() {
			methodOut := inspectpkg.Method{
				Name:            method.GetName(),
				Description:     method.GetDescription(),
				InputType:       method.GetInputType(),
				OutputType:      method.GetOutputType(),
				ClientStreaming: method.GetClientStreaming(),
				ServerStreaming: method.GetServerStreaming(),
				ExampleInput:    method.GetExampleInput(),
			}
			for _, field := range method.GetInputFields() {
				methodOut.InputFields = append(methodOut.InputFields, inspectFieldFromProto(field))
			}
			for _, field := range method.GetOutputFields() {
				methodOut.OutputFields = append(methodOut.OutputFields, inspectFieldFromProto(field))
			}
			serviceOut.Methods = append(serviceOut.Methods, methodOut)
		}
		out.Services = append(out.Services, serviceOut)
	}
	for _, skill := range doc.GetSkills() {
		out.Skills = append(out.Skills, inspectpkg.Skill{
			Name:        skill.GetName(),
			Description: skill.GetDescription(),
			When:        skill.GetWhen(),
			Steps:       append([]string(nil), skill.GetSteps()...),
		})
	}
	for _, sequence := range doc.GetSequences() {
		sequenceOut := inspectpkg.Sequence{
			Name:        sequence.GetName(),
			Description: sequence.GetDescription(),
			Steps:       append([]string(nil), sequence.GetSteps()...),
		}
		for _, param := range sequence.GetParams() {
			sequenceOut.Params = append(sequenceOut.Params, inspectpkg.SequenceParam{
				Name:        param.GetName(),
				Description: param.GetDescription(),
				Required:    param.GetRequired(),
				Default:     param.GetDefault(),
			})
		}
		out.Sequences = append(out.Sequences, sequenceOut)
	}
	return out
}

func inspectFieldFromProto(field *opv1.InspectField) inspectpkg.Field {
	if field == nil {
		return inspectpkg.Field{}
	}
	out := inspectpkg.Field{
		Name:         field.GetName(),
		Type:         field.GetType(),
		Number:       field.GetNumber(),
		Description:  field.GetDescription(),
		Label:        field.GetLabel(),
		MapKeyType:   field.GetMapKeyType(),
		MapValueType: field.GetMapValueType(),
		Required:     field.GetRequired(),
		Example:      field.GetExample(),
	}
	for _, nested := range field.GetNestedFields() {
		out.NestedFields = append(out.NestedFields, inspectFieldFromProto(nested))
	}
	for _, enumValue := range field.GetEnumValues() {
		out.EnumValues = append(out.EnumValues, inspectpkg.EnumValue{
			Name:        enumValue.GetName(),
			Number:      enumValue.GetNumber(),
			Description: enumValue.GetDescription(),
		})
	}
	return out
}

func doResultToProto(result *dopkg.Result) *opv1.SequenceResult {
	if result == nil {
		return nil
	}
	out := &opv1.SequenceResult{
		Holon:           result.Holon,
		Sequence:        result.Sequence,
		Description:     result.Description,
		Dir:             result.Dir,
		DryRun:          result.DryRun,
		ContinueOnError: result.ContinueOnError,
		Params:          make(map[string]string, len(result.Params)),
	}
	for key, value := range result.Params {
		out.Params[key] = value
	}
	for _, step := range result.Steps {
		out.Steps = append(out.Steps, &opv1.SequenceStepResult{
			Index:   int32(step.Index),
			Command: step.Command,
			Output:  step.Output,
			Success: step.Success,
			Error:   step.Error,
		})
	}
	return out
}

func templateEntryToProto(entry scaffold.Entry) *opv1.TemplateEntry {
	out := &opv1.TemplateEntry{
		Name:        entry.Name,
		Description: entry.Description,
		Lang:        entry.Lang,
	}
	for _, param := range entry.Params {
		out.Params = append(out.Params, &opv1.TemplateParam{
			Name:        param.Name,
			Description: param.Description,
			Default:     param.Default,
			Required:    param.Required,
		})
	}
	return out
}

func dependencyToProto(dep opmod.Dependency) *opv1.Dependency {
	return &opv1.Dependency{
		Path:      dep.Path,
		Version:   dep.Version,
		CachePath: dep.CachePath,
	}
}

func updatedDependencyToProto(dep opmod.UpdatedDependency) *opv1.UpdatedDependency {
	return &opv1.UpdatedDependency{
		Path:       dep.Path,
		OldVersion: dep.OldVersion,
		NewVersion: dep.NewVersion,
	}
}

func edgeToProto(edge opmod.Edge) *opv1.DependencyEdge {
	return &opv1.DependencyEdge{
		From:    edge.From,
		To:      edge.To,
		Version: edge.Version,
	}
}
