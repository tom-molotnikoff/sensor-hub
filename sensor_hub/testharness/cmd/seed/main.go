package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"example/sensorHub/testharness/seed"
)

func main() {
	out := flag.String("out", "", "path of the seed database file to write")
	printVersion := flag.Bool("version", false, "print the seed version and exit")
	force := flag.Bool("force", false, "regenerate even when the file is already at the current seed version")
	readings := flag.Int("readings", seed.Default.Readings, "number of readings to write")
	flag.Parse()

	if *printVersion {
		fmt.Println(seed.Version)
		return
	}

	if *out == "" {
		fmt.Fprintln(os.Stderr, "-out is required")
		os.Exit(2)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if !*force && seed.IsCurrent(*out) {
		logger.Info("seed database already at the current version", "path", *out, "version", seed.Version)
		return
	}

	shape := seed.Default
	shape.Readings = *readings

	if err := seed.Generate(context.Background(), *out, shape, logger); err != nil {
		logger.Error("failed to generate seed database", "error", err)
		os.Exit(1)
	}
}
