package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/joseph-gunnarsson/go-sim-local/internal/config"
)

const (
	encClearLine = "\033[2K"
	encMoveStart = "\r"
)

type ReplicaLog struct {
	ReplicaName string
	Message     string
}

type Runner struct {
	CMDS            map[string]*CMDEXEC
	logs            []ReplicaLog
	isolatedReplica string
	sync.RWMutex
}

type CMDEXEC struct {
	Cmd *exec.Cmd
}

func NewRunner() *Runner {
	return &Runner{
		CMDS: make(map[string]*CMDEXEC),
		logs: make([]ReplicaLog, 0),
	}
}

func (r *Runner) AddCmd(name string, cmd *exec.Cmd) {
	r.Lock()
	defer r.Unlock()
	r.CMDS[name] = &CMDEXEC{
		Cmd: cmd,
	}
}

func (r *Runner) RemoveCmd(name string) {
	r.Lock()
	defer r.Unlock()
	delete(r.CMDS, name)
}

func (r *Runner) Clear() {
	fmt.Print("\033[H\033[2J")
}

func (r *Runner) isolate(args []string) error {
	if len(args) < 2 {
		return errors.New("isolate command requires a replica name")
	}
	replicaName := strings.TrimSpace(args[1])
	if _, ok := r.CMDS[replicaName]; !ok {
		return fmt.Errorf("replica %s not found", replicaName)
	}

	r.Lock()
	r.isolatedReplica = replicaName
	r.Unlock()
	r.Clear()
	r.IsolateLogs(replicaName)
	log.Printf("[Sim] Isolated logs for replica %s", replicaName)
	return nil

}

func (r *Runner) ReadInput() error {
	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		args := strings.Split(input, " ")
		switch strings.TrimSpace(args[0]) {
		case "isolate":
			err := r.isolate(args)
			if err != nil {
				fmt.Println("Error:", err)
			}
		case "showall":
			r.Lock()
			r.isolatedReplica = ""
			r.Unlock()
			r.Clear()
			log.Println("[Sim] Showing logs for all replicas")
		default:
			fmt.Println("[Sim] Unknown command")
		}
	}

	return nil
}

func (r *Runner) ReadCurrentCommand() string {
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadByte()
	if err != nil {
		return ""
	}
	return string(input)

}

func (r *Runner) StartService(ctx context.Context, cfgService config.Service) error {
	r.Clear()

	for i := 0; i < cfgService.Replicas; i++ {
		replicaName := fmt.Sprintf("%s-%d", cfgService.Name, i+1)

		cmd := exec.Command("go", "run", cfgService.Path)

		cmd.Env = os.Environ()
		for k, v := range cfgService.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", cfgService.StartPort+i))

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("[%s] Error creating stdout pipe: %w", replicaName, err)
		}
		stderrout, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("[%s] Error creating stderr pipe: %w", replicaName, err)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("[%s] Error starting command: %w", replicaName, err)
		}

		r.AddCmd(replicaName, cmd)

		fmt.Printf("[Sim] Starting replica %s\n", replicaName)

		go r.outputLogs(replicaName, "STDOUT", stdout)
		go r.outputLogs(replicaName, "STDERR", stderrout)

		go r.waitForExit(replicaName, cmd)
	}

	return nil
}

func (r *Runner) waitForExit(replicaName string, cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.Printf("[Sim] Replica %s exited with error: %s", replicaName, err)
	} else {
		log.Printf("[Sim] Replica %s exited successfully", replicaName)
	}

	r.RemoveCmd(replicaName)
}

func (r *Runner) outputLogs(replicaName, logType string, pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		m := scanner.Text()
		output := fmt.Sprintf("[%s][%s] %s", logType, replicaName, m)

		r.addLog(replicaName, output)
		r.UpdateLatestLog()
	}
}
func printLogAboveInput(msg string) {
	fmt.Print("\r\x1b[1L")
	fmt.Println(msg)
}

func (r *Runner) UpdateLatestLog() {
	r.RLock()
	defer r.RUnlock()

	if len(r.logs) == 0 {
		return
	}

	latestLog := r.logs[len(r.logs)-1]

	if r.isolatedReplica == "" || r.isolatedReplica == latestLog.ReplicaName {
		printLogAboveInput(latestLog.Message)
	}

}
func (r *Runner) addLog(replicaName, logLine string) {
	r.Lock()
	defer r.Unlock()
	r.logs = append(r.logs, ReplicaLog{
		ReplicaName: replicaName,
		Message:     logLine,
	})
}

func (r *Runner) GetLogs() []ReplicaLog {
	r.RLock()
	defer r.RUnlock()
	var filtered []ReplicaLog
	for _, l := range r.logs {
		if l.ReplicaName == r.isolatedReplica {
			filtered = append(filtered, l)
		}
	}
	return filtered
}

func (r *Runner) IsolateLogs(replicaName string) {
	r.RLock()
	defer r.RUnlock()
	r.Clear()
	for _, l := range r.logs {
		if l.ReplicaName != replicaName {
			log.Println(l.Message)
		}
	}
}

func (r *Runner) stopReplica(replicaName string) {
	r.Lock()
	replica, ok := r.CMDS[replicaName]
	if ok {
		delete(r.CMDS, replicaName)
	}
	r.Unlock()

	if !ok {
		return
	}

	if err := replica.Cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		log.Printf("[Sim] Error killing replica %s: %s", replicaName, err)
	}
}

func (r *Runner) ShutdownAll() {
	r.RLock()
	replicas := make([]string, 0, len(r.CMDS))
	for name := range r.CMDS {
		replicas = append(replicas, name)
	}
	r.RUnlock()

	for _, name := range replicas {
		r.stopReplica(name)
	}
}
