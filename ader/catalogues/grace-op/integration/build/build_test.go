package build_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildOpBootstrap(t *testing.T) {
	// Configuration du bac à sable respectant l'isolation (ADER_RUN_ARTIFACTS)
	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")

	// Phase 1 : `go build` classique dans notre bac à sable (Génération 1)
	gen1Bin := filepath.Join(opBin, "op-gen1")
	t.Logf("Phase 1: Construction native (Génération 1) vers %s", gen1Bin)
	cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, "../../../../../holons/grace-op/cmd/op")
	if out, err := cmdGen1.CombinedOutput(); err != nil {
		t.Fatalf("Echec de la phase 1 (go build natif): %v\nSortie: %s", err, string(out))
	}

	// Préparation de l'environnement isolé commun (OPPATH/OPBIN)
	envVars := append(os.Environ(), "OPPATH="+opPath, "OPBIN="+opBin)

	// Phase 2 : Exécution du binaire Génération 1 pour compiler `op` (Gen2)
	// (équivalent au `go run <op> build op` demandé)
	t.Log("Phase 2: Génération 1 construit Génération 2 (op build op --install)")
	cmdGen2 := exec.Command(gen1Bin, "build", "op", "--install", "--symlink", "--root", "../../../../..")
	cmdGen2.Env = envVars
	if out, err := cmdGen2.CombinedOutput(); err != nil {
		t.Fatalf("Echec de la phase 2 (Gen1 build Gen2): %v\nSortie: %s", err, string(out))
	}
	
	// Le binaire symlinké construit par op s'appelle "op"
	gen2Bin := filepath.Join(opBin, "op")
	if stat, err := os.Stat(gen2Bin); os.IsNotExist(err) || stat.Size() == 0 {
		t.Fatalf("La phase 2 n'a pas produit le binaire attendu %s", gen2Bin)
	}

	// Phase 3 : Le binaire Gen2 compile à son tour `op` (Gen3)
	// C'est le cas ultime le plus autoréférentiel !
	t.Log("Phase 3: Génération 2 construit Génération 3 (op build op)")
	cmdGen3 := exec.Command(gen2Bin, "build", "op", "--install", "--symlink", "--root", "../../../../..")
	cmdGen3.Env = envVars
	if out, err := cmdGen3.CombinedOutput(); err != nil {
		t.Fatalf("Echec de la phase 3 (Gen2 build Gen3): %v\nSortie: %s", err, string(out))
	}

	t.Log("Le cycle complet de bootstrap autoréférentiel a réussi.")
}
