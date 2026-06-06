package fdc

import (
	"context"
	"testing"

	"caltui/internal/domain"
)

func TestParseSearch(t *testing.T) {
	data := []byte(`{"foods":[
	  {"fdcId":111,"description":"Cheerios","dataType":"Branded","brandName":"General Mills",
	   "servingSize":28,"servingSizeUnit":"g","householdServingFullText":"1 cup",
	   "foodNutrients":[
	     {"nutrientNumber":"208","unitName":"KCAL","value":367},
	     {"nutrientNumber":"203","unitName":"G","value":12},
	     {"nutrientNumber":"205","unitName":"G","value":73},
	     {"nutrientNumber":"204","unitName":"G","value":6}]},
	  {"fdcId":222,"description":"No energy","dataType":"Branded",
	   "foodNutrients":[{"nutrientNumber":"203","value":5}]},
	  {"fdcId":333,"description":"Atwater only","dataType":"Foundation",
	   "foodNutrients":[{"nutrientNumber":"957","unitName":"KCAL","value":150},
	                    {"nutrientNumber":"203","unitName":"G","value":3}]}
	]}`)
	foods, err := parseSearch(data)
	if err != nil {
		t.Fatal(err)
	}
	// The no-energy food is dropped.
	if len(foods) != 2 {
		t.Fatalf("got %d foods, want 2", len(foods))
	}

	c := foods[0]
	if c.Name != "Cheerios" || c.Brand != "General Mills" {
		t.Errorf("food0 = %+v", c)
	}
	if c.Per100g != (domain.Macros{Kcal: 367, Protein: 12, Carbs: 73, Fat: 6}) {
		t.Errorf("macros = %+v", c.Per100g)
	}
	if c.Source != domain.SourceOnlineUSDA || c.FDCID == nil || *c.FDCID != 111 {
		t.Errorf("source/fdcid wrong: %+v", c)
	}
	if c.ServingSize != 28 || c.ServingUnit != domain.UnitGram || c.Household != "1 cup" {
		t.Errorf("serving wrong: %+v", c)
	}

	// Atwater (957) energy is used when 208 is absent.
	if foods[1].Per100g.Kcal != 150 {
		t.Errorf("atwater energy = %g, want 150", foods[1].Per100g.Kcal)
	}
}

func TestEnergyIgnoresKJ(t *testing.T) {
	byNum := map[string]searchNutrient{"208": {Number: "208", UnitName: "kJ", Value: 1500}}
	if _, ok := energyKcal(byNum); ok {
		t.Error("kJ energy should not be accepted as kcal")
	}
	byNum["208"] = searchNutrient{Number: "208", UnitName: "KCAL", Value: 360}
	if v, ok := energyKcal(byNum); !ok || v != 360 {
		t.Errorf("kcal energy = %g ok=%v, want 360", v, ok)
	}
}

func TestSearchRequiresKey(t *testing.T) {
	if _, err := New("").Search(context.Background(), "anything", 5); err == nil {
		t.Error("expected an error when no API key is configured")
	}
}
