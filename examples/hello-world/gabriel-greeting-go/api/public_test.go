package api_test

import (
	"testing"

	"gabriel-greeting-go/api"
	pb "gabriel-greeting-go/gen/go/greeting/v1"
)

func TestListLanguagesIncludesEnglish(t *testing.T) {
	response, err := api.ListLanguages()
	if err != nil {
		t.Fatalf("ListLanguages() error = %v", err)
	}
	if len(response.GetLanguages()) == 0 {
		t.Fatal("ListLanguages() returned no languages")
	}

	for _, language := range response.GetLanguages() {
		if language.GetCode() == "en" {
			if language.GetName() != "English" {
				t.Fatalf("English name = %q, want %q", language.GetName(), "English")
			}
			if language.GetNative() != "English" {
				t.Fatalf("English native = %q, want %q", language.GetNative(), "English")
			}
			return
		}
	}

	t.Fatal(`ListLanguages() did not include language code "en"`)
}

func TestSayHelloUsesRequestedLanguage(t *testing.T) {
	response, err := api.SayHello(&pb.SayHelloRequest{
		Name:     "Alice",
		LangCode: "fr",
	})
	if err != nil {
		t.Fatalf("SayHello() error = %v", err)
	}
	if response.GetGreeting() != "Bonjour Alice" {
		t.Fatalf("Greeting = %q, want %q", response.GetGreeting(), "Bonjour Alice")
	}
	if response.GetLanguage() != "French" {
		t.Fatalf("Language = %q, want %q", response.GetLanguage(), "French")
	}
	if response.GetLangCode() != "fr" {
		t.Fatalf("LangCode = %q, want %q", response.GetLangCode(), "fr")
	}
}

func TestSayHelloFallsBackToEnglishDefaultName(t *testing.T) {
	response, err := api.SayHello(&pb.SayHelloRequest{
		LangCode: "unknown",
	})
	if err != nil {
		t.Fatalf("SayHello() error = %v", err)
	}
	if response.GetGreeting() != "Hello Mary" {
		t.Fatalf("Greeting = %q, want %q", response.GetGreeting(), "Hello Mary")
	}
	if response.GetLanguage() != "English" {
		t.Fatalf("Language = %q, want %q", response.GetLanguage(), "English")
	}
	if response.GetLangCode() != "en" {
		t.Fatalf("LangCode = %q, want %q", response.GetLangCode(), "en")
	}
}
