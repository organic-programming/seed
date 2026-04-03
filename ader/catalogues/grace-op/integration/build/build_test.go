package build_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestBuildDryRun(t *testing.T) {
	// Exécuter `go run` sur le point d'entrée de op avec la commande build en dry-run
	cmd := exec.Command("go", "run", "../../../../../holons/grace-op/cmd/op", "build", "clem-ader", "--dry-run")
	
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Echec de l'exécution de 'op build clem-ader --dry-run': %v\nSortie: %s", err, out.String())
	}

	output := out.String()
	
	// Vérification des indicateurs d'un build plan
	if !strings.Contains(output, "Operation: build") {
		t.Errorf("Le résultat devrait contenir 'Operation: build', obtenu: %s", output)
	}
	if !strings.Contains(output, "Holon: clem-ader") {
		t.Errorf("Le résultat devrait identifier le holon 'clem-ader', obtenu: %s", output)
	}
	if !strings.Contains(output, "dry run — no commands executed") {
		t.Errorf("Le résultat devrait indiquer que c'est un dry-run, obtenu: %s", output)
	}
}
