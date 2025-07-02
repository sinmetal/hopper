package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/spanner"
	"github.com/sinmetal/hopper"
)

func main() {
	ctx := context.Background()

	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		log.Fatal("GCP_PROJECT environment variable must be set.")
	}
	instanceID := os.Getenv("SPANNER_INSTANCE")
	if instanceID == "" {
		log.Fatal("SPANNER_INSTANCE environment variable must be set.")
	}
	databaseID := os.Getenv("SPANNER_DATABASE")
	if databaseID == "" {
		log.Fatal("SPANNER_DATABASE environment variable must be set.")
	}

	dsn := fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectID, instanceID, databaseID)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World")
	})
	http.HandleFunc("/singers/random-insert", sh.RandomInsert)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
