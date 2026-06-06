// Command etl builds the bundled offline food database from USDA FoodData
// Central SR Legacy CSVs. It downloads + extracts the dataset if needed, joins
// food / food_nutrient / food_portion, keeps per-100g energy + the three macros
// (with an Atwater energy fallback), and writes a gzipped SQLite seed embedded
// by internal/seed.
//
// Run via `make etl`. It is a build-time tool and is NOT part of the app binary.
package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"caltui/internal/domain"
	"caltui/internal/store"
)

const (
	srLegacyURL      = "https://fdc.nal.usda.gov/fdc-datasets/FoodData_Central_sr_legacy_food_csv_2018-04.zip"
	extractedDirName = "FoodData_Central_sr_legacy_food_csv_2018-04"

	// FDC nutrient ids we keep.
	nEnergy    = 1008 // Energy (kcal)
	nEnergyAtw = 2047 // Energy (Atwater General Factors), kcal
	nEnergyAtS = 2048 // Energy (Atwater Specific Factors), kcal
	nProtein   = 1003
	nFat       = 1004
	nCarbs     = 1005
)

func main() {
	src := flag.String("src", "", "directory of extracted USDA CSVs (auto-download if empty)")
	out := flag.String("out", "internal/seed/foods.sqlite.gz", "output gzipped sqlite seed path")
	dataDir := flag.String("data", "tools/etl/_data", "working directory for downloads")
	flag.Parse()

	csvDir := *src
	if csvDir == "" {
		d, err := ensureData(*dataDir)
		if err != nil {
			log.Fatalf("obtaining USDA data: %v", err)
		}
		csvDir = d
	}

	foods, err := buildFoods(csvDir)
	if err != nil {
		log.Fatalf("building foods: %v", err)
	}
	log.Printf("assembled %d foods", len(foods))

	if err := writeSeed(foods, *out); err != nil {
		log.Fatalf("writing seed: %v", err)
	}
	fi, _ := os.Stat(*out)
	log.Printf("wrote %s (%d KB)", *out, fi.Size()/1024)
}

// --- nutrient accumulation ---

type nutrientSet struct {
	energy, energyAtwGen, energyAtwSpec, protein, fat, carbs float64
}

func (n nutrientSet) kcal() float64 {
	switch {
	case n.energy > 0:
		return n.energy
	case n.energyAtwGen > 0:
		return n.energyAtwGen
	default:
		return n.energyAtwSpec
	}
}

func buildFoods(dir string) ([]domain.Food, error) {
	names, err := parseFoods(dir)
	if err != nil {
		return nil, fmt.Errorf("food.csv: %w", err)
	}
	nutrients, err := parseNutrients(dir)
	if err != nil {
		return nil, fmt.Errorf("food_nutrient.csv: %w", err)
	}
	portions, err := parsePortions(dir)
	if err != nil {
		return nil, fmt.Errorf("food_portion.csv: %w", err)
	}

	foods := make([]domain.Food, 0, len(names))
	for fdc, name := range names {
		ns, ok := nutrients[fdc]
		if !ok {
			continue
		}
		kcal := ns.kcal()
		if kcal <= 0 {
			continue // require an energy value
		}
		id := int64(fdc)
		f := domain.Food{
			Source:  domain.SourceOfflineUSDA,
			FDCID:   &id,
			Name:    name,
			Per100g: domain.Macros{Kcal: kcal, Protein: ns.protein, Carbs: ns.carbs, Fat: ns.fat},
		}
		if p, ok := portions[fdc]; ok {
			f.ServingSize = p.grams
			f.ServingUnit = domain.UnitGram
			f.Household = household(p.label, p.grams)
		}
		foods = append(foods, f)
	}
	return foods, nil
}

func parseFoods(dir string) (map[int]string, error) {
	f, r, idx, err := openCSV(filepath.Join(dir, "food.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fdcI, typeI, descI := idx["fdc_id"], idx["data_type"], idx["description"]
	out := make(map[int]string)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if rec[typeI] != "sr_legacy_food" {
			continue
		}
		id, err := strconv.Atoi(rec[fdcI])
		if err != nil {
			continue
		}
		out[id] = strings.Clone(strings.TrimSpace(rec[descI]))
	}
	return out, nil
}

func parseNutrients(dir string) (map[int]*nutrientSet, error) {
	f, r, idx, err := openCSV(filepath.Join(dir, "food_nutrient.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fdcI, nutI, amtI := idx["fdc_id"], idx["nutrient_id"], idx["amount"]
	out := make(map[int]*nutrientSet)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		nid, err := strconv.Atoi(rec[nutI])
		if err != nil {
			continue
		}
		switch nid {
		case nEnergy, nEnergyAtw, nEnergyAtS, nProtein, nFat, nCarbs:
		default:
			continue
		}
		fdc, err := strconv.Atoi(rec[fdcI])
		if err != nil {
			continue
		}
		amt, err := strconv.ParseFloat(rec[amtI], 64)
		if err != nil {
			continue
		}
		ns := out[fdc]
		if ns == nil {
			ns = &nutrientSet{}
			out[fdc] = ns
		}
		switch nid {
		case nEnergy:
			ns.energy = amt
		case nEnergyAtw:
			ns.energyAtwGen = amt
		case nEnergyAtS:
			ns.energyAtwSpec = amt
		case nProtein:
			ns.protein = amt
		case nFat:
			ns.fat = amt
		case nCarbs:
			ns.carbs = amt
		}
	}
	return out, nil
}

type portion struct {
	seq   int
	grams float64
	label string
}

func parsePortions(dir string) (map[int]portion, error) {
	f, r, idx, err := openCSV(filepath.Join(dir, "food_portion.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fdcI, seqI, gwI := idx["fdc_id"], idx["seq_num"], idx["gram_weight"]
	modI, descI := idx["modifier"], idx["portion_description"]
	out := make(map[int]portion)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		gw, err := strconv.ParseFloat(rec[gwI], 64)
		if err != nil || gw <= 0 {
			continue
		}
		fdc, err := strconv.Atoi(rec[fdcI])
		if err != nil {
			continue
		}
		seq, _ := strconv.Atoi(rec[seqI])
		label := strings.TrimSpace(rec[modI])
		if label == "" {
			label = strings.TrimSpace(rec[descI])
		}
		// Keep the lowest seq_num portion (the primary one).
		if cur, ok := out[fdc]; !ok || seq < cur.seq {
			out[fdc] = portion{seq: seq, grams: gw, label: strings.Clone(label)}
		}
	}
	return out, nil
}

func household(label string, grams float64) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "serving"
	}
	return fmt.Sprintf("%s (%g g)", label, grams)
}

// --- seed output ---

func writeSeed(foods []domain.Food, outPath string) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	tmpDB := outPath + ".build.sqlite"
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(tmpDB + suffix)
	}
	s, err := store.Open(tmpDB)
	if err != nil {
		return err
	}
	if _, err := s.InsertFoods(foods); err != nil {
		_ = s.Close()
		return err
	}
	if err := s.Checkpoint(); err != nil {
		_ = s.Close()
		return err
	}
	if err := s.Close(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := gzipFile(tmpDB, outPath); err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(tmpDB + suffix)
	}
	return nil
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	zw, _ := gzip.NewWriterLevel(out, gzip.BestCompression)
	if _, err := io.Copy(zw, in); err != nil {
		_ = zw.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return out.Close()
}

// --- download / extract ---

func ensureData(dataDir string) (string, error) {
	csvDir := filepath.Join(dataDir, extractedDirName)
	if fileExists(filepath.Join(csvDir, "food.csv")) {
		log.Printf("using cached CSVs at %s", csvDir)
		return csvDir, nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", err
	}
	zipPath := filepath.Join(dataDir, "sr_legacy.zip")
	if !fileExists(zipPath) {
		log.Printf("downloading %s", srLegacyURL)
		if err := download(srLegacyURL, zipPath); err != nil {
			return "", err
		}
	}
	log.Printf("extracting %s", zipPath)
	if err := unzip(zipPath, dataDir); err != nil {
		return "", err
	}
	if !fileExists(filepath.Join(csvDir, "food.csv")) {
		return "", fmt.Errorf("food.csv not found after extraction in %s", csvDir)
	}
	return csvDir, nil
}

func download(url, dst string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func unzip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, file := range zr.File {
		target := filepath.Join(dest, file.Name) //nolint:gosec // trusted USDA archive
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractOne(file, target); err != nil {
			return err
		}
	}
	return nil
}

func extractOne(file *zip.File, target string) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc) //nolint:gosec // trusted USDA archive
	return err
}

// --- csv helpers ---

func openCSV(path string) (*os.File, *csv.Reader, map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.ReuseRecord = true
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		_ = f.Close()
		return nil, nil, nil, err
	}
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	return f, r, idx, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
