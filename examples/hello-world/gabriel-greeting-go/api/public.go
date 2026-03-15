package api

import (
	"fmt"
	"strings"

	pb "gabriel-greeting-go/gen/go/greeting/v1"
	"gabriel-greeting-go/internal"
)

// ListLanguages returns all available greeting languages.
func ListLanguages() (*pb.ListLanguagesResponse, error) {
	// This public implementation does not expose the internal implementation.
	langs := make([]*pb.Language, len(internal.Greetings))
	for i, g := range internal.Greetings {
		langs[i] = &pb.Language{
			Code:   g.LangCode,
			Name:   g.LangEnglish,
			Native: g.LangNative,
		}
	}
	return &pb.ListLanguagesResponse{Languages: langs}, nil
}

// SayHello greets the user in the requested language.

func SayHello(req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
	g := internal.Lookup(req.LangCode)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = g.DefaultName
	}
	return &pb.SayHelloResponse{
		Greeting: fmt.Sprintf(g.Template, name),
		Language: g.LangEnglish,
		LangCode: g.LangCode,
	}, nil
}
