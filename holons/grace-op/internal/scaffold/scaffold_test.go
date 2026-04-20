package scaffold

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestListIncludesOnlyShippedTemplates(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name)
	}

	want := []string{"coax-flutter", "coax-swiftui"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
}

func TestGenerateUnknownTemplateFails(t *testing.T) {
	root := t.TempDir()

	_, err := Generate("composite-go-web", "orbit-console", GenerateOptions{Dir: root})
	if err == nil {
		t.Fatal("Generate() succeeded, want unknown template error")
	}
	if !strings.Contains(err.Error(), `unknown template "composite-go-web"`) {
		t.Fatalf("Generate() error = %v, want unknown template", err)
	}
}

func TestGenerateCoaxFlutterTemplateRendersScaffold(t *testing.T) {
	root := t.TempDir()

	result, err := Generate("coax-flutter", "henri-nobody", GenerateOptions{Dir: root})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	if result.Template != "coax-flutter" {
		t.Fatalf("result.Template = %q, want %q", result.Template, "coax-flutter")
	}

	pubspecPath := filepath.Join(root, "henri-nobody", "app", "pubspec.yaml")
	pubspec, err := os.ReadFile(pubspecPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", pubspecPath, err)
	}
	for _, expected := range []string{
		`name: henri_nobody_app`,
		`path: ../../organism_kits/flutter`,
		`path: ../../sdk/dart-holons`,
	} {
		if !strings.Contains(string(pubspec), expected) {
			t.Fatalf("pubspec missing %q:\n%s", expected, string(pubspec))
		}
	}

	appPath := filepath.Join(root, "henri-nobody", "app", "lib", "src", "app.dart")
	appData, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", appPath, err)
	}
	if !strings.Contains(string(appData), "sample.v1.SampleHolon/SetGreeting") {
		t.Fatalf("app scaffold missing demo COAX wiring:\n%s", string(appData))
	}

	packagerPath := filepath.Join(root, "henri-nobody", "app", "tool", "package_desktop.dart")
	packagerData, err := os.ReadFile(packagerPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", packagerPath, err)
	}
	if !strings.Contains(string(packagerData), "// TODO: list your member holons") {
		t.Fatalf("packager missing member TODO:\n%s", string(packagerData))
	}
}

func TestGenerateCoaxSwiftUITemplateRendersScaffold(t *testing.T) {
	root := t.TempDir()

	result, err := Generate("coax-swiftui", "henri-nobody", GenerateOptions{Dir: root})
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	if result.Template != "coax-swiftui" {
		t.Fatalf("result.Template = %q, want %q", result.Template, "coax-swiftui")
	}

	projectPath := filepath.Join(root, "henri-nobody", "project.yml")
	projectData, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", projectPath, err)
	}
	for _, expected := range []string{
		`{{ if .Hardened }}`,
		`CODE_SIGN_ENTITLEMENTS: App/HenriNobody.entitlements`,
		`product: OrganismKit`,
	} {
		if !strings.Contains(string(projectData), expected) {
			t.Fatalf("project.yml missing %q:\n%s", expected, string(projectData))
		}
	}

	packagePath := filepath.Join(root, "henri-nobody", "Modules", "Package.swift")
	packageData, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", packagePath, err)
	}
	for _, expected := range []string{
		`fileURLWithPath: "../../organism_kits/swiftui"`,
		`fileURLWithPath: "../../sdk/swift-holons"`,
		`name: "OrganismKit"`,
	} {
		if !strings.Contains(string(packageData), expected) {
			t.Fatalf("Package.swift missing %q:\n%s", expected, string(packageData))
		}
	}

	processPath := filepath.Join(root, "henri-nobody", "Modules", "Sources", "AppKit", "AppHolonManager.swift")
	processData, err := os.ReadFile(processPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", processPath, err)
	}
	if !strings.Contains(string(processData), "sample.v1.SampleHolon/SetGreeting") {
		t.Fatalf("AppHolonManager missing demo COAX method:\n%s", string(processData))
	}
}
