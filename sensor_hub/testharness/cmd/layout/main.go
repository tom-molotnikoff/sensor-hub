//go:build integration

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"example/sensorHub/testharness"
)

func main() {
	seedPath := flag.String("seed", "", "seed database to start from, left untouched")
	uiDir := flag.String("ui", "", "directory holding the built UI")
	addr := flag.String("addr", "127.0.0.1:4173", "address to listen on")
	fixtures := flag.String("fixtures", "", "comma-separated layout fixtures to load")
	flag.Parse()

	if *seedPath == "" || *uiDir == "" {
		fmt.Fprintln(os.Stderr, "-seed and -ui are required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	env, cleanup, err := testharness.StartLayoutServer(ctx, testharness.LayoutOptions{
		SeedPath:   *seedPath,
		UI:         os.DirFS(*uiDir),
		ListenAddr: *addr,
		Fixtures:   splitList(*fixtures),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start layout harness: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	fmt.Printf("layout harness listening on %s\n", env.ServerURL)
	<-ctx.Done()
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}
