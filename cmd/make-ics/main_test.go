package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeSimpleWorkbook(path string) error {
	wb := excelize.NewFile()
	sheet := wb.GetSheetName(0)
	wb.SetCellValue(sheet, "A1", "03-apr-26")
	wb.SetCellValue(sheet, "B1", "A")
	wb.SetCellValue(sheet, "C1", "10:00 uur")
	return wb.SaveAs(path)
}

func TestRun_CreatesIcsAndLocalises(t *testing.T) {
	tmp := t.TempDir()
	xlsx := filepath.Join(tmp, "input.xlsx")
	if err := writeSimpleWorkbook(xlsx); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	cfg := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfg, []byte("timezone: Europe/Amsterdam\nlocale: nl_NL\nshift_type:\n  A: {}\n"), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	// locales dir is ../../locales relative to cmd/make-ics (go test cwd = package dir)
	localesDir := filepath.Join("..", "..", "locales")

	// capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runErr := Run([]string{"-c", cfg, xlsx}, localesDir)
	w.Close()
	outBytes, _ := io.ReadAll(r)
	os.Stdout = old

	if runErr != nil {
		t.Fatalf("Run failed: %v", runErr)
	}
	out := string(outBytes)
	if len(strings.TrimSpace(out)) == 0 {
		t.Fatalf("expected some output from Run, got empty string")
	}
	icsPath := filepath.Join(tmp, "input.ics")
	if _, err := os.Stat(icsPath); err != nil {
		t.Fatalf("expected %s to exist: %v", icsPath, err)
	}
}

func TestRun_ExitsOnBadConfig(t *testing.T) {
	tmp := t.TempDir()
	xlsx := filepath.Join(tmp, "input.xlsx")
	if err := writeSimpleWorkbook(xlsx); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	cfg := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfg, []byte("locale: nl_NL\n"), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if err := Run([]string{"-c", cfg, xlsx}, filepath.Join("..", "..", "locales")); err == nil {
		t.Fatalf("expected error for invalid config")
	}
}
