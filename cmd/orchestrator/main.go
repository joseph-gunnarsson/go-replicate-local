package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joseph-gunnarsson/go-replicate-local/internal/config"
	ui "github.com/joseph-gunnarsson/go-replicate-local/internal/interface"
	"github.com/joseph-gunnarsson/go-replicate-local/internal/lb"
	"github.com/joseph-gunnarsson/go-replicate-local/internal/runner"
)

type logWriter struct {
	logChan chan<- string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	msg = strings.TrimSuffix(msg, "\n")
	w.logChan <- msg
	return len(p), nil
}

func main() {
	configFile := flag.String("config", "simulation.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logChan, cmdChan, program := ui.Setup()

	log.SetOutput(&logWriter{logChan: logChan})
	log.SetFlags(0) 

	r := runner.NewRunner()
	r.SetLogCallback(func(msg string) {
		logChan <- msg
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {

		for _, svc := range cfg.Services {
			if err := r.StartService(ctx, svc); err != nil {
				ui.SendLog(program, ui.FormatError(fmt.Sprintf("Failed to start %s: %v", svc.Name, err)))
			} else {
				ui.SendLog(program, ui.FormatSuccess(fmt.Sprintf("Started service: %s (%d replicas)", svc.Name, svc.Replicas)))
			}
		}

		go func() {
			if err := lb.StartLB(ctx, cfg); err != nil {
				ui.SendLog(program, ui.FormatError(fmt.Sprintf("Load Balancer failed: %v", err)))
				cancel()
			}
		}()
		ui.SendLog(program, ui.FormatSuccess(fmt.Sprintf("Load balancer started on port %d", cfg.LBPort)))

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		<-sigChan
		ui.SendLog(program, "\n[Sim] Received shutdown signal, stopping all replicas...")
		cancel() 
		r.ShutdownAll()
		program.Quit()
	}()

	go func() {
		for cmd := range cmdChan {
			handleCommand(cmd, r, program, cancel)
		}
	}()

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
	
	cancel()
	r.ShutdownAll()
}

func handleCommand(input string, r *runner.Runner, program *ui.Program, cancel context.CancelFunc) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "help":
		ui.SendLog(program, ui.HelpText())

	case "list":
		replicas := r.ListReplicas()
		ui.SendLog(program, ui.FormatReplicaList(replicas))

	case "isolate":
		if len(args) < 1 {
			ui.SendLog(program, ui.FormatError("Usage: isolate <replica-name>"))
			return
		}
		replicaName := args[0]
		replicas := r.ListReplicas()
		found := false
		for _, rn := range replicas {
			if rn == replicaName {
				found = true
				break
			}
		}
		if !found {
			ui.SendLog(program, ui.FormatError(fmt.Sprintf("Replica '%s' not found", replicaName)))
			return
		}
		r.SetIsolatedReplica(replicaName)
		program.Send(ui.SetFilterMsg(replicaName))
		ui.SendLog(program, ui.FormatSuccess(fmt.Sprintf("Now showing logs only from: %s", replicaName)))

	case "showall":
		r.SetIsolatedReplica("")
		program.Send(ui.SetFilterMsg(""))
		ui.SendLog(program, ui.FormatSuccess("Now showing logs from all replicas"))

	case "kill":
		if len(args) < 1 {
			ui.SendLog(program, ui.FormatError("Usage: kill <replica-name>"))
			return
		}
		replicaName := args[0]
		if err := r.StopReplica(replicaName); err != nil {
			ui.SendLog(program, ui.FormatError(err.Error()))
		} else {
			ui.SendLog(program, ui.FormatSuccess(fmt.Sprintf("Stopped replica: %s", replicaName)))
		}

	case "quit", "exit":
		ui.SendLog(program, "[Sim] Shutting down...")
		r.ShutdownAll()
		cancel()
		program.Quit()

	default:
		ui.SendLog(program, ui.FormatError(fmt.Sprintf("Unknown command: %s (type 'help' for available commands)", cmd)))
	}
}
