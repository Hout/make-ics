package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/internal/defaultcfg"
	"github.com/jeroen/make-ics-go/pkg/config"
	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/ics"
	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

//go:embed static/index.html
var indexHTML []byte

const (
	maxUploadBytes        = 10 << 20 // 10 MB
	defaultAdvanceMinutes = 30
)

func main() {
	port := flag.String("port", "8080", "TCP port to listen on")
	cfgPath := flag.String("config", "", "Optional path to a config.yaml override (uses built-in default if empty)")
	flag.Parse()

	cfg, lines, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", serveIndex)
	mux.HandleFunc("POST /convert", makeConvertHandler(cfg, lines))

	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Listening on http://localhost:%s", *port)

	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("Shutting down…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func loadConfig(path string) (model.Config, config.LineMap, error) {
	var (
		cfg   model.Config
		lines config.LineMap
		err   error
	)
	if path != "" {
		cfg, lines, err = config.LoadConfig(path)
		if err != nil {
			return cfg, nil, fmt.Errorf("loading %q: %w", path, err)
		}
	}
	if config.IsEmpty(cfg) {
		cfg, lines, err = config.LoadConfigFromBytes(defaultcfg.DefaultConfig)
		if err != nil {
			return cfg, nil, fmt.Errorf("loading embedded config: %w", err)
		}
	}
	if err := config.ValidateConfig(cfg, path, lines); err != nil {
		return cfg, nil, err
	}
	return cfg, lines, nil
}

func serveIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

func makeConvertHandler(cfg model.Config, lines config.LineMap) http.HandlerFunc {
	loc, err := i18n.NewLocalizer(cfg.Locale)
	if err != nil {
		// Panic at startup rather than silently failing per request.
		panic(fmt.Sprintf("i18n init: %v", err))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			http.Error(w, "Upload te groot of ongeldig (max 10 MB).", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Geen bestand ontvangen.", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if ext := strings.ToLower(filepath.Ext(header.Filename)); ext != ".xlsx" {
			http.Error(w, "Alleen .xlsx-bestanden zijn toegestaan.", http.StatusBadRequest)
			return
		}

		data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes))
		if err != nil {
			http.Error(w, "Leesfout bij het uploaden.", http.StatusInternalServerError)
			log.Printf("read upload: %v", err)
			return
		}

		wb, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			http.Error(w, "Kon het xlsx-bestand niet openen. Is het een geldig rooster?", http.StatusBadRequest)
			log.Printf("open workbook: %v", err)
			return
		}
		defer wb.Close()

		events, err := pipeline.IterEvents(
			wb,
			defaultAdvanceMinutes,
			cfg.Timezone,
			cfg.ShiftType,
			cfg.Seasons,
			cfg.Exceptions,
			lines,
			loc,
		)
		if err != nil {
			http.Error(w, "Fout bij het omzetten van het rooster.", http.StatusInternalServerError)
			log.Printf("IterEvents: %v", err)
			return
		}

		icsName := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename)) + ".ics"
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, icsName))
		if err := ics.WriteCalendarWriter(w, header.Filename, events); err != nil {
			// Headers already sent; just log.
			log.Printf("WriteCalendarWriter: %v", err)
		}
	}
}
