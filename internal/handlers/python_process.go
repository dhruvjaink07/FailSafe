package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	pb "github.com/dhruvjaink07/failsafe/internal/proto"
	"google.golang.org/grpc"
)

var (
	pythonCmdMu sync.Mutex
	pythonCmd   *exec.Cmd
	// supervision control
	superviseOnce sync.Once
	superviseStop chan struct{}
	shutdownReq   bool
	superviseMu   sync.Mutex
)

// EnsurePythonServer starts the Python gRPC server as a supervised subprocess.
// It will start a supervisor goroutine (only once) that restarts the process
// on unexpected exit with exponential backoff. This function waits until the
// Health RPC reports ok or the startup timeout elapses.
func EnsurePythonServer(ctx context.Context) error {
	pythonCmdMu.Lock()
	if pythonCmd != nil && pythonCmd.Process != nil {
		// already started
		pythonCmdMu.Unlock()
		return nil
	}
	pythonCmdMu.Unlock()

	pythonExec := os.Getenv("PYTHON_EXEC")
	if pythonExec == "" {
		pythonExec = "python"
	}
	script := os.Getenv("PYTHON_SCRIPT")
	if script == "" {
		script = "internal/Prod/grpc_server.py"
	}
	// If PYTHON_MODULE is set, we'll run `python -m <module>` which allows
	// package-relative imports (recommended when the server uses relative imports).
	module := os.Getenv("PYTHON_MODULE")

	addr := os.Getenv("PYTHON_GRPC_ADDR")
	if addr == "" {
		addr = "localhost:50051"
	}
	timeoutSec := 10
	if v := os.Getenv("PYTHON_STARTUP_TIMEOUT"); v != "" {
		if t, err := fmt.Sscanf(v, "%d", &timeoutSec); t == 0 || err != nil {
			timeoutSec = 10
		}
	}

	// Start supervisor once; it will manage starting/restarting the process
	superviseOnce.Do(func() {
		// Run a quick diagnostic using the configured python executable so
		// logs show which Python binary and package versions are available.
		diagCode := `import sys, traceback
print("PYTHON:", sys.executable)
try:
    import lightgbm as lgb
    print("lightgbm:", lgb.__version__)
except Exception as e:
    print("lightgbm import failed:", e)
try:
    import psycopg2
    print("psycopg2: ok")
except Exception as e:
    print("psycopg2 import failed:", e)
`
		if out, err := exec.Command(pythonExec, "-c", diagCode).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "python diag failed: %v\noutput:\n%s\n", err, string(out))
		} else {
			fmt.Fprintf(os.Stderr, "python diag output:\n%s\n", string(out))
		}
		superviseStop = make(chan struct{})
		go func() {
			backoff := time.Second
			for {
				superviseMu.Lock()
				if shutdownReq {
					superviseMu.Unlock()
					return
				}
				superviseMu.Unlock()

				var cmd *exec.Cmd
				if module != "" {
					cmd = exec.CommandContext(ctx, pythonExec, "-m", module)
				} else {
					cmd = exec.CommandContext(ctx, pythonExec, script)
				}
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				if err := cmd.Start(); err != nil {
					fmt.Fprintf(os.Stderr, "failed to start python grpc server: %v\n", err)
					select {
					case <-time.After(backoff):
						backoff *= 2
						if backoff > 30*time.Second {
							backoff = 30 * time.Second
						}
						continue
					case <-superviseStop:
						return
					}
				}

				// register process
				pythonCmdMu.Lock()
				pythonCmd = cmd
				pythonCmdMu.Unlock()

				// wait for process exit or stop signal
				exitCh := make(chan error, 1)
				go func() { exitCh <- cmd.Wait() }()

				// Wait for exit or stop
				select {
				case err := <-exitCh:
					if err != nil {
						fmt.Fprintf(os.Stderr, "python grpc server exited: %v\n", err)
					} else {
						fmt.Fprintln(os.Stderr, "python grpc server exited")
					}
					// clear registered cmd
					pythonCmdMu.Lock()
					if pythonCmd == cmd {
						pythonCmd = nil
					}
					pythonCmdMu.Unlock()

					// if shutdown requested, stop supervising
					superviseMu.Lock()
					if shutdownReq {
						superviseMu.Unlock()
						return
					}
					superviseMu.Unlock()

					// backoff before restart
					select {
					case <-time.After(backoff):
						backoff *= 2
						if backoff > 30*time.Second {
							backoff = 30 * time.Second
						}
						continue
					case <-superviseStop:
						return
					}
				case <-superviseStop:
					// stop requested: try to kill process then exit
					_ = cmd.Process.Kill()
					return
				}
			}
		}()
	})

	// Poll health until ready or timeout
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		if err := checkPythonHealth(ctx, addr, 2*time.Second); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("python grpc server did not become healthy within %d seconds", timeoutSec)
}

// StopPythonServer attempts to terminate the subprocess and stop supervision.
func StopPythonServer() error {
	superviseMu.Lock()
	shutdownReq = true
	superviseMu.Unlock()

	// signal supervisor to stop
	if superviseStop != nil {
		select {
		case <-superviseStop:
		default:
			close(superviseStop)
		}
	}

	pythonCmdMu.Lock()
	defer pythonCmdMu.Unlock()
	if pythonCmd == nil || pythonCmd.Process == nil {
		return nil
	}
	err := pythonCmd.Process.Kill()
	pythonCmd = nil
	return err
}

func checkPythonHealth(ctx context.Context, addr string, timeout time.Duration) error {
	ctxDial, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctxDial, addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewInferenceClient(conn)
	ctxCall, cancelCall := context.WithTimeout(ctx, 2*time.Second)
	defer cancelCall()
	resp, err := client.Health(ctxCall, &pb.HealthRequest{})
	if err != nil {
		return err
	}
	status := resp.GetStatus()
	if status != "ok" {
		// Allow degraded health to be accepted in development/testing when
		// explicitly enabled via env var `PYTHON_ALLOW_DEGRADED=1`.
		allowDegraded := os.Getenv("PYTHON_ALLOW_DEGRADED")
		if allowDegraded == "1" || allowDegraded == "true" || allowDegraded == "True" {
			if status == "degraded" {
				return nil
			}
		}
		return fmt.Errorf("health status: %s (%s)", status, resp.GetDetails())
	}
	return nil
}
