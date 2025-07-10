package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/spanner"
	"github.com/sinmetal/hopper"
	"github.com/sinmetal/hopper/internal/trace"
	"github.com/sinmetalcraft/gcpbox/metadata/cloudrun"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	ctx := context.Background()

	spanner.EnableOpenTelemetryMetrics()

	projectID, err := cloudrun.ProjectID()
	if err != nil {
		log.Fatalf("failed to get project id: %v", err)
	}

	shutdown, err := trace.InitTracer(projectID)
	if err != nil {
		log.Fatalf("failed to init tracer: %v", err)
	}
	defer shutdown()

	spannerProjectID := os.Getenv("SPANNER_PROJECT_ID")
	if spannerProjectID == "" {
		log.Fatal("SPANNER_PROJECT_ID environment variable must be set.")
	}

	instanceID := os.Getenv("SPANNER_INSTANCE")
	if instanceID == "" {
		log.Fatal("SPANNER_INSTANCE environment variable must be set.")
	}
	databaseID := os.Getenv("SPANNER_DATABASE")
	if databaseID == "" {
		log.Fatal("SPANNER_DATABASE environment variable must be set.")
	}

	dsn := fmt.Sprintf("projects/%s/instances/%s/databases/%s", spannerProjectID, instanceID, databaseID)
	sc, err := spanner.NewClient(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer sc.Close()

	ss, err := hopper.NewSingersStore(ctx, sc)
	if err != nil {
		log.Fatal(err)
	}

	sh := hopper.NewSingersHandler(ss)

	http.Handle("/", otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World")
	}), "root"))
	http.Handle("/singers/random-insert", otelhttp.NewHandler(http.HandlerFunc(sh.RandomInsert), "random-insert"))
	http.Handle("/singers/random-update", otelhttp.NewHandler(http.HandlerFunc(sh.RandomUpdate), "random-update"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
