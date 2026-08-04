package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/manual"
)

func TestItemSetDefinitionCoverageReportsIndexAndFiligreeSetsSeparately(t *testing.T) {
	t.Parallel()
	filigreeData, err := json.Marshal([]dataset.FiligreeSet{{Name: "Defined Filigree"}, {Name: "Missing Filigree"}})
	if err != nil {
		t.Fatal(err)
	}
	master := dataset.Master{
		Items: []dataset.ItemRecord{{Item: dataset.ItemData{
			Name: "Set Item", SetBonus: []dataset.SetBonusOut{{Name: "Defined Index"}, {Name: "Missing Index"}},
		}}},
		Files: []dataset.CanonicalFile{{
			MasterFile: dataset.MasterFile{Kind: "filigree-sets", Path: "filigreeSets.json"}, Data: filigreeData,
		}},
	}
	manualRoot := t.TempDir()
	data := `[{"name":"Defined Index","bonuses":[]},{"name":"Defined Filigree","bonuses":[]}]`
	if err := os.WriteFile(filepath.Join(manualRoot, manual.ItemSetEnchantmentsFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err := itemSetDefinitionCoverage(master, manualRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if warnings[0].source != "setBonusIndex" || !warnings[0].manualFound || !reflect.DeepEqual(warnings[0].missingSetNames, []string{"Missing Index"}) {
		t.Fatalf("index warning = %#v", warnings[0])
	}
	if warnings[1].source != "filigreeSets" || !reflect.DeepEqual(warnings[1].missingSetNames, []string{"Missing Filigree"}) {
		t.Fatalf("filigree warning = %#v", warnings[1])
	}
}

func TestItemSetDefinitionCoverageRejectsInvalidManualDefinitions(t *testing.T) {
	t.Parallel()
	manualRoot := t.TempDir()
	data := `[{"name":"Duplicate","bonuses":[]},{"name":"Duplicate","bonuses":[]}]`
	if err := os.WriteFile(filepath.Join(manualRoot, manual.ItemSetEnchantmentsFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := itemSetDefinitionCoverage(dataset.Master{}, manualRoot)
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("error = %v", err)
	}
}
