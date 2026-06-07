// Package fdc is an online food.Provider backed by the USDA FoodData Central
// API. It requires a free API key (https://fdc.nal.usda.gov/api-key-signup);
// without one, callers should fall back to the offline provider.
//
// Nutrient values from the search endpoint are treated as per-100g. Energy is
// taken from nutrientNumber 208 (kcal), falling back to the Atwater numbers
// 957/958 when 208 is absent, always preferring KCAL units.
package fdc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"caltui/internal/domain"
)

const defaultBase = "https://api.nal.usda.gov/fdc/v1"

// Client queries USDA FoodData Central.
type Client struct {
	apiKey string
	base   string
	http   *http.Client
}

// New returns a client using the given API key.
func New(apiKey string) *Client {
	return &Client{apiKey: apiKey, base: defaultBase, http: &http.Client{Timeout: 8 * time.Second}}
}

// Name identifies this provider.
func (c *Client) Name() string { return "usda" }

// Validate confirms the API key works by issuing a tiny live search. It returns
// nil when the key is accepted, or an error (rejected key, rate limit, network).
func (c *Client) Validate(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("fdc: no API key")
	}
	_, err := c.Search(ctx, "apple", 1)
	return err
}

type searchRequest struct {
	Query    string   `json:"query"`
	DataType []string `json:"dataType"`
	PageSize int      `json:"pageSize"`
}

// Search queries the foods/search endpoint and normalizes the results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]domain.Food, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("fdc: no API key configured")
	}
	if limit <= 0 {
		limit = 25
	}
	body, err := json.Marshal(searchRequest{
		Query:    query,
		DataType: []string{"Branded", "Foundation", "SR Legacy"},
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	endpoint := c.base + "/foods/search?api_key=" + url.QueryEscape(c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("fdc: rate limited (HTTP 429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fdc: search failed: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseSearch(data)
}

// --- response parsing / normalization ---

type searchResponse struct {
	Foods []searchFood `json:"foods"`
}

type searchFood struct {
	FDCID                    int64            `json:"fdcId"`
	Description              string           `json:"description"`
	DataType                 string           `json:"dataType"`
	BrandName                string           `json:"brandName"`
	BrandOwner               string           `json:"brandOwner"`
	ServingSize              float64          `json:"servingSize"`
	ServingSizeUnit          string           `json:"servingSizeUnit"`
	HouseholdServingFullText string           `json:"householdServingFullText"`
	FoodNutrients            []searchNutrient `json:"foodNutrients"`
}

type searchNutrient struct {
	Number   string  `json:"nutrientNumber"`
	Name     string  `json:"nutrientName"`
	UnitName string  `json:"unitName"`
	Value    float64 `json:"value"`
}

// parseSearch unmarshals a search response and normalizes each food to per-100g.
func parseSearch(data []byte) ([]domain.Food, error) {
	var r searchResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("fdc: decoding response: %w", err)
	}
	out := make([]domain.Food, 0, len(r.Foods))
	for _, sf := range r.Foods {
		f, ok := normalize(sf)
		if ok {
			out = append(out, f)
		}
	}
	return out, nil
}

func normalize(sf searchFood) (domain.Food, bool) {
	// A search result can list the same nutrientNumber more than once (a
	// per-100g entry plus a per-serving one). The per-100g value comes first, so
	// keep the FIRST occurrence and ignore later duplicates.
	byNum := make(map[string]searchNutrient, len(sf.FoodNutrients))
	for _, n := range sf.FoodNutrients {
		if _, exists := byNum[n.Number]; !exists {
			byNum[n.Number] = n
		}
	}
	kcal, ok := energyKcal(byNum)
	if !ok {
		return domain.Food{}, false // no usable energy value
	}
	macro := func(num string) float64 {
		if n, ok := byNum[num]; ok {
			return n.Value
		}
		return 0
	}
	brand := sf.BrandName
	if brand == "" {
		brand = sf.BrandOwner
	}
	id := sf.FDCID
	f := domain.Food{
		Source:    domain.SourceOnlineUSDA,
		FDCID:     &id,
		Name:      sf.Description,
		Brand:     brand,
		Per100g:   domain.Macros{Kcal: kcal, Protein: macro("203"), Carbs: macro("205"), Fat: macro("204")},
		Household: sf.HouseholdServingFullText,
	}
	// FDC uses unit codes like "GRM"/"MLT" as well as "g"/"ml".
	switch strings.ToUpper(sf.ServingSizeUnit) {
	case "G", "GRM":
		f.ServingSize, f.ServingUnit = sf.ServingSize, domain.UnitGram
	case "ML", "MLT":
		f.ServingSize, f.ServingUnit = sf.ServingSize, domain.UnitMilliliter
	}
	return f, true
}

// energyKcal picks the kcal energy value, preferring 208 then the Atwater
// numbers 957/958, and only accepting KCAL units (ignoring kJ).
func energyKcal(byNum map[string]searchNutrient) (float64, bool) {
	for _, num := range []string{"208", "957", "958"} {
		if n, ok := byNum[num]; ok && isKcal(n.UnitName) {
			return n.Value, true
		}
	}
	return 0, false
}

func isKcal(unit string) bool {
	switch unit {
	case "KCAL", "kcal", "Kcal", "":
		return true
	default:
		return false
	}
}
