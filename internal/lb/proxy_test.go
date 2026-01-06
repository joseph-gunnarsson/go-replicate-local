package lb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/joseph-gunnarsson/go-replicate-local/internal/config"
)

func startMockServer(port int, id string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Response from %s", id)
	})
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	go func() {
		server.ListenAndServe()
	}()
	return server
}

func TestRoundRobinLoadBalancer(t *testing.T) {
	startPort := 50000
	lbPort := 50005
	replicas := 2

	srv1 := startMockServer(startPort, "backend-1")
	srv2 := startMockServer(startPort+1, "backend-2")
	defer srv1.Close()
	defer srv2.Close()

	time.Sleep(100 * time.Millisecond)

	cfg := config.Config{
		LBPort: lbPort,
		Services: map[string]config.Service{
			"test-service": {
				Name:        "test-service",
				StartPort:   startPort,
				Replicas:    replicas,
				RoutePrefix: "/test",
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go func() {
		err := StartLB(ctx, cfg)
		if err != nil {
			fmt.Printf("LB Error: %v\n", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	lbURL := fmt.Sprintf("http://localhost:%d/test", lbPort)

	client := &http.Client{}

	for i := 0; i < 4; i++ {
		resp, err := client.Get(lbURL)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		
		fmt.Printf("Req %d: %s\n", i, string(body))
	}
}
