package grpcclient

import (
	"testing"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// FieldOptions is a generated receiver with explicit scalar presence. The
// synthetic Describe input must deliver an explicit false distinctly from an
// omitted value, including inside a nested message, without reflection.
func TestDescribePreservesExplicitDefaultScalarOnWire(t *testing.T) {
	input, _, err := buildDescribeDescriptors(&holonsv1.MethodDoc{
		InputType: "presence.Request", OutputType: "presence.Response",
		InputFields: []*holonsv1.FieldDoc{
			{Name: "name", Type: "string", Number: 1},
			{Name: "number", Type: "int32", Number: 3},
			{
				Name: "options", Type: "presence.Options", Number: 8,
				NestedFields: []*holonsv1.FieldDoc{
					{Name: "deprecated", Type: "bool", Number: 3},
					{Name: "lazy", Type: "bool", Number: 5},
				},
			}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, json string
		want       *bool
	}{
		{"false", `{"name":"","number":0,"options":{"deprecated":false}}`, proto.Bool(false)},
		{"true", `{"options":{"deprecated":true}}`, proto.Bool(true)},
		{"omitted", `{"options":{}}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := dynamicpb.NewMessage(input)
			if err := protojson.Unmarshal([]byte(tc.json), message); err != nil {
				t.Fatal(err)
			}
			wire, err := proto.Marshal(message)
			if err != nil {
				t.Fatal(err)
			}
			var receiver descriptorpb.FieldDescriptorProto
			if err := proto.Unmarshal(wire, &receiver); err != nil {
				t.Fatal(err)
			}
			if receiver.Options == nil {
				t.Fatal("nested options were omitted")
			}
			got := receiver.Options.Deprecated
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("deprecated presence/value = %v, want %v; wire=%x", got, tc.want, wire)
			}
			if receiver.Options.Lazy != nil {
				t.Fatal("omitted scalar gained presence")
			}
			if tc.name == "false" {
				if receiver.Name == nil || receiver.GetName() != "" || receiver.Number == nil || receiver.GetNumber() != 0 {
					t.Fatalf("explicit empty string/zero lost presence: %v", &receiver)
				}
			} else if receiver.Name != nil || receiver.Number != nil {
				t.Fatal("omitted string/number gained presence")
			}
		})
	}
}
