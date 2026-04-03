package build_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildOp(t *testing.T) {
	// Utilisation des artifacts Ader si disponible, sinon TempDir
	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")

	// Exécuter `go run` sur op pour construire `op` avec installation
	// `op run` n'est pas utilisé directement ici pour s'amorcer, on utilise `go run ... build op --install`
	cmd := exec.Command("go", "run", "../../../../../holons/grace-op/cmd/op", "build", "op", "--install", "--symlink", "--root", "../../../../..")
	
	// Injecter OPPATH et OPBIN pour isoler le build de la machine hôte
	cmd.Env = append(os.Environ(), 
		"OPPATH="+opPath,
		"OPBIN="+opBin,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Echec de l'exécution de 'op build op --install': %v\nSortie: %s", err, out.String())
	}

	output := out.String()
	
	// Vérification des indicateurs d'un build plan abouti
	if !strings.Contains(output, "Operation: install") && !strings.Contains(output, "Operation: build") {
		t.Errorf("Le résultat devrait indiquer une opération d'installation ou build, obtenu: %s", output)
	}
	if !strings.Contains(output, "Holon: op") {
		t.Errorf("Le résultat devrait identifier le holon 'op', obtenu: %s", output)
	}
	
	// Vérifier que le binaire symlinké est bien dans OPBIN
	expectedSymlink := filepath.Join(opBin, "op")
	if _, err := os.Stat(expectedSymlink); os.IsNotExist(err) {
		t.Errorf("Le symlink attendu n'a pas été trouvé dans OPBIN : %s", expectedSymlink)
	}
}
