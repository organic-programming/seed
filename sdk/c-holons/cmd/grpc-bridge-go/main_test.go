package main

import (
	"os"
	"path/filepath"
	"testing"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestParseArgsAllowsStaticDescribeWithoutProtoDir(t *testing.T) {
	t.Parallel()

	cfg, err := parseArgs([]string{
		"--backend", "/tmp/backend",
		"--describe-static", "/tmp/describe.json",
	})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if cfg.protoDir != "" {
		t.Fatalf("expected protoDir to stay empty, got %q", cfg.protoDir)
	}
	if cfg.describeStatic != "/tmp/describe.json" {
		t.Fatalf("unexpected describeStatic: %q", cfg.describeStatic)
	}
}

func TestLoadMethodRegistryFromStaticDescribe(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	describePath := filepath.Join(root, "describe.json")
	response := testDescribeResponse()

	data, err := protojson.Marshal(response)
	if err != nil {
		t.Fatalf("marshal static describe: %v", err)
	}
	if err := os.WriteFile(describePath, data, 0644); err != nil {
		t.Fatalf("write static describe: %v", err)
	}

	methods, err := loadMethodRegistry("", describePath)
	if err != nil {
		t.Fatalf("loadMethodRegistry: %v", err)
	}

	sayHello := methods["/greeting.v1.GreetingService/SayHello"]
	if sayHello == nil {
		t.Fatalf("missing SayHello method in registry: %v", methods)
	}
	if got := sayHello.Input().Fields().ByName(protoreflect.Name("lang_code")); got == nil || got.Kind() != protoreflect.StringKind {
		t.Fatalf("SayHello input lang_code kind = %v, want string", got)
	}
	if got := sayHello.Output().Fields().ByName(protoreflect.Name("greeting")); got == nil || got.Kind() != protoreflect.StringKind {
		t.Fatalf("SayHello output greeting kind = %v, want string", got)
	}

	listLanguages := methods["/greeting.v1.GreetingService/ListLanguages"]
	if listLanguages == nil {
		t.Fatalf("missing ListLanguages method in registry: %v", methods)
	}
	languagesField := listLanguages.Output().Fields().ByName(protoreflect.Name("languages"))
	if languagesField == nil {
		t.Fatalf("ListLanguages output is missing languages field")
	}
	if !languagesField.IsList() {
		t.Fatalf("languages field should be repeated")
	}
	if got := string(languagesField.Message().FullName()); got != "greeting.v1.Language" {
		t.Fatalf("languages field message = %q, want greeting.v1.Language", got)
	}
	if got := languagesField.Message().Fields().ByName(protoreflect.Name("native")); got == nil || got.Kind() != protoreflect.StringKind {
		t.Fatalf("Language.native kind = %v, want string", got)
	}
}

func testDescribeResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Services: []*holonsv1.ServiceDoc{
			{
				Name: "greeting.v1.GreetingService",
				Methods: []*holonsv1.MethodDoc{
					{
						Name:       "ListLanguages",
						InputType:  "greeting.v1.ListLanguagesRequest",
						OutputType: "greeting.v1.ListLanguagesResponse",
						OutputFields: []*holonsv1.FieldDoc{
							{
								Name:   "languages",
								Type:   "greeting.v1.Language",
								Number: 1,
								Label:  holonsv1.FieldLabel_FIELD_LABEL_REPEATED,
								NestedFields: []*holonsv1.FieldDoc{
									{
										Name:   "code",
										Type:   "string",
										Number: 1,
										Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
									},
									{
										Name:   "name",
										Type:   "string",
										Number: 2,
										Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
									},
									{
										Name:   "native",
										Type:   "string",
										Number: 3,
										Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
									},
								},
							},
						},
					},
					{
						Name:       "SayHello",
						InputType:  "greeting.v1.SayHelloRequest",
						OutputType: "greeting.v1.SayHelloResponse",
						InputFields: []*holonsv1.FieldDoc{
							{
								Name:   "name",
								Type:   "string",
								Number: 1,
								Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
							},
							{
								Name:   "lang_code",
								Type:   "string",
								Number: 2,
								Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
							},
						},
						OutputFields: []*holonsv1.FieldDoc{
							{
								Name:   "greeting",
								Type:   "string",
								Number: 1,
								Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
							},
							{
								Name:   "language",
								Type:   "string",
								Number: 2,
								Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
							},
							{
								Name:   "lang_code",
								Type:   "string",
								Number: 3,
								Label:  holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
							},
						},
					},
				},
			},
		},
	}
}
