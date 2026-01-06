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
	"sync"
	"syscall"

	"github.com/joseph-gunnarsson/go-replicate-local/internal/config"
)



type ReplicaLog struct {
	ReplicaName string
	Message     string
}

type Runner struct {
	CMDS            map[string]*CMDEXEC
	logs            []ReplicaLog
	isolatedReplica string
	logCallback     func(string)
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





func (r *Runner) StartService(ctx context.Context, cfgService config.Service) error {

	for i := 0; i < cfgService.Replicas; i++ {
		replicaName := fmt.Sprintf("%s-%d", cfgService.Name, i+1)

		cmd := exec.Command("go", "run", cfgService.Path)

		cmd.Env = os.Environ()
		for k, v := range cfgService.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", cfgService.StartPort+i))

		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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


func (r *Runner) UpdateLatestLog() {
	r.RLock()
	defer r.RUnlock()

	if len(r.logs) == 0 {
		return
	}

	latestLog := r.logs[len(r.logs)-1]

	if r.isolatedReplica == "" || r.isolatedReplica == latestLog.ReplicaName {
		if r.logCallback != nil {
			r.logCallback(latestLog.Message)
		}
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



func (r *Runner) StopReplica(replicaName string) error {
	r.Lock()
	replica, ok := r.CMDS[replicaName]
	if ok {
		delete(r.CMDS, replicaName)
	}
	r.Unlock()

	if !ok {
		return fmt.Errorf("replica %s not found", replicaName)
	}

	pgid, err := syscall.Getpgid(replica.Cmd.Process.Pid)
	if err == nil {
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			log.Printf("[Sim] Error killing replica group %s: %s", replicaName, err)
		}
	} else {
		if err := replica.Cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			log.Printf("[Sim] Error killing replica %s: %s", replicaName, err)
			return err
		}
	}
	
	log.Printf("[Sim] Stopped replica %s", replicaName)
	return nil
}

func (r *Runner) ListReplicas() []string {
	r.RLock()
	defer r.RUnlock()
	replicas := make([]string, 0, len(r.CMDS))
	for name := range r.CMDS {
		replicas = append(replicas, name)
	}
	return replicas
}

func (r *Runner) GetIsolatedReplica() string {
	r.RLock()
	defer r.RUnlock()
	return r.isolatedReplica
}

func (r *Runner) SetIsolatedReplica(name string) {
	r.Lock()
	defer r.Unlock()
	r.isolatedReplica = name
}

func (r *Runner) SetLogCallback(cb func(string)) {
	r.Lock()
	defer r.Unlock()
	r.logCallback = cb
}

func (r *Runner) ShutdownAll() {
	r.RLock()
	replicas := make([]string, 0, len(r.CMDS))
	for name := range r.CMDS {
		replicas = append(replicas, name)
	}
	r.RUnlock()

	for _, name := range replicas {
		r.StopReplica(name)
	}
}
