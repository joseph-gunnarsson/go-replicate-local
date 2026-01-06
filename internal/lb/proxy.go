package lb

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/joseph-gunnarsson/go-replicate-local/internal/config"
)

type Backend struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy
}

type ServiceLB struct {
	Backends []Backend
	current  uint64
	mu       sync.RWMutex
}

func (s *ServiceLB) NextBackend() *Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Backends) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&s.current, 1) % uint64(len(s.Backends))
	return &s.Backends[idx]
}

func (s *ServiceLB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := s.NextBackend()
	if backend == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	log.Printf("[LB] Routing %s to %s", r.URL.Path, backend.URL.String())
	backend.ReverseProxy.ServeHTTP(w, r)
}

func StartLB(ctx context.Context, cfg config.Config) error {
	mux := http.NewServeMux()
	registered := make(map[string]bool)

	for _, svc := range cfg.Services {
		if registered[svc.RoutePrefix] {
			log.Printf("[LB] Warning: Route prefix %q for service %q is already registered. Skipping.", svc.RoutePrefix, svc.Name)
			continue
		}
		
		slb := &ServiceLB{
			Backends: make([]Backend, 0, svc.Replicas),
		}

		for i := 0; i < svc.Replicas; i++ {
			port := svc.StartPort + i
			targetURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
			if err != nil {
				return fmt.Errorf("failed to parse backend url: %w", err)
			}

			proxy := httputil.NewSingleHostReverseProxy(targetURL)

			slb.Backends = append(slb.Backends, Backend{
				URL:          targetURL,
				ReverseProxy: proxy,
			})
		}


		routePattern := svc.RoutePrefix
		if !strings.HasSuffix(routePattern, "/") {
			routePattern += "/"
		}

		mux.Handle(routePattern, http.StripPrefix(svc.RoutePrefix, slb))
		registered[svc.RoutePrefix] = true
		
		log.Printf("[LB] Registered service %s at %s with %d replicas", svc.Name, routePattern, svc.Replicas)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.LBPort),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		log.Println("[LB] Shutting down load balancer...")
		server.Shutdown(context.Background())
	}()

	log.Printf("[LB] Starting Load Balancer on port %d", cfg.LBPort)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}
