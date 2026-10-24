// Command launcher starts the whole stack for local use: the Go API (which
// also serves the built frontend) and a free Cloudflare tunnel. The console
// then streams the logs of every service; Ctrl+C stops them all.
//
// The tunnel URL is injected into the backend (PUBLIC_BASE_URL) so invite links
// point at the public address instead of localhost.
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	backendPort = "3001"
)

var tunnelURLPattern = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type proc struct {
	name    string
	cmd     *exec.Cmd
	logPath string
}

func main() {
	setupConsole()

	root, err := repoRoot()
	if err != nil {
		fatal("locate project root: %v", err)
	}

	logDir := filepath.Join(root, ".launcher")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		fatal("create log dir: %v", err)
	}
	// Fresh logs per run: the console and the files show this run only.
	for _, name := range []string{"tunnel", "backend", "build"} {
		if err := os.WriteFile(filepath.Join(logDir, name+".log"), nil, 0o644); err != nil {
			fatal("reset log %s: %v", name, err)
		}
	}

	for _, bin := range []struct{ name, hint string }{
		{"node", "install Node.js 20+"},
		{"npm", "install Node.js 20+"},
		{"cloudflared", "install cloudflared (https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)"},
		{"ffmpeg", "install ffmpeg (needed for transcoding)"},
	} {
		if _, err := exec.LookPath(bin.name); err != nil {
			fmt.Fprintf(os.Stderr, "  ! %-12s not found - %s\n", bin.name, bin.hint)
		}
	}

	// 1. Build the frontend once: the backend serves the bundle, so the
	// tunnel does not have to push Vite's dev server to the viewer.
	buildFrontend(root, logDir)

	// 2. Tunnel first: the backend needs its URL for the invite links.
	tunnelURL, tunnel := startTunnel(root, logDir)

	// 3. Backend, told which public origin to use for invite links.
	buildBackend(root, logDir)
	backend := startBackend(root, logDir, tunnelURL)

	procs := []*proc{tunnel, backend}

	// Ctrl+C / SIGTERM must stop the children as well.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		shutdown(procs)
		os.Exit(0)
	}()

	waitFor("http://localhost:"+backendPort+"/health", 30*time.Second)

	// The LAN address lets other devices on the same network open the app.
	if ip := localIP(); ip != "" {
		fmt.Printf("LAN:  http://%s:%s\n", ip, backendPort)
	}
	openBrowser("http://localhost:" + backendPort)
	fmt.Println("Press Ctrl+C to stop everything.")
	fmt.Println()

	streamLogs(root)
}

// localIP returns the address of this machine on the local network. The UDP
// dial picks the interface of the default route (nothing is sent), so virtual
// adapters do not win.
func localIP() string {
	if conn, err := net.Dial("udp4", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if ua, ok := conn.LocalAddr().(*net.UDPAddr); ok && ua.IP.To4() != nil {
			return ua.IP.String()
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

// tailer prints new lines of one log file to the console.
type tailer struct {
	name    string
	path    string
	offset  int64
	partial string
}

// poll writes whatever was appended to the log file since the last call.
func (t *tailer) poll() {
	f, err := os.Open(t.path)
	if err != nil {
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return
	}
	if st.Size() < t.offset {
		t.offset = 0 // the file was truncated
	}
	if st.Size() == t.offset {
		return
	}
	buf := make([]byte, st.Size()-t.offset)
	n, _ := f.ReadAt(buf, t.offset)
	t.offset += int64(n)
	t.partial += string(buf[:n])
	lines := strings.Split(t.partial, "\n")
	t.partial = lines[len(lines)-1]
	for _, line := range lines[:len(lines)-1] {
		fmt.Printf("[%s] %s\n", t.name, line)
	}
}

// streamLogs prints the logs of every service to the console until the process
// is stopped.
func streamLogs(root string) {
	tails := []*tailer{
		{name: "tunnel", path: logPath(root, "tunnel")},
		{name: "backend", path: logPath(root, "backend")},
		{name: "build", path: logPath(root, "build")},
	}
	for {
		for _, t := range tails {
			t.poll()
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func startTunnel(root, logDir string) (string, *proc) {
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return "", nil
	}
	// http2 instead of QUIC: on flaky uplinks QUIC keeps re-handshaking and
	// every page load waits for it.
	cmd := exec.Command("cloudflared", "tunnel", "--no-autoupdate", "--protocol", "http2", "--url", "http://localhost:"+backendPort)
	p := &proc{name: "tunnel", cmd: cmd, logPath: filepath.Join(logDir, "tunnel.log")}
	pipe, err := cmd.StderrPipe()
	if err != nil {
		fatal("tunnel stderr: %v", err)
	}
	// Both streams go through the same pipe so the log file stays in order.
	cmd.Stdout = cmd.Stderr
	logFile, err := os.OpenFile(p.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fatal("open tunnel log: %v", err)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "  ! could not start cloudflared: %v\n", err)
		return "", nil
	}

	// The quick tunnel prints its URL once it is up: scan the log and tee it.
	urlCh := make(chan string, 1)
	go func() {
		defer logFile.Close()
		buf := make([]byte, 4096)
		seen := ""
		for {
			n, err := pipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				seen += chunk
				logFile.WriteString(chunk)
				if m := tunnelURLPattern.FindString(seen); m != "" {
					select {
					case urlCh <- m:
					default:
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()

	select {
	case url := <-urlCh:
		return url, p
	case <-time.After(45 * time.Second):
		return "", p
	}
}

// buildFrontend produces the production bundle the backend serves.
func buildFrontend(root, logDir string) {
	logFile, err := os.OpenFile(filepath.Join(logDir, "build.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fatal("open build log: %v", err)
	}
	defer logFile.Close()

	fmt.Println("Building the frontend...")
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = filepath.Join(root, "frontend")
	cmd.Stdout = io.MultiWriter(os.Stderr, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	if err := cmd.Run(); err != nil {
		fatal("build frontend: %v", err)
	}
}

func buildBackend(root, logDir string) {
	cmd := exec.Command("go", "build", "-o", filepath.Join(logDir, serverName()), "./cmd/server")
	cmd.Dir = filepath.Join(root, "backend")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("build backend: %v", err)
	}
}

func serverName() string {
	if runtime.GOOS == "windows" {
		return "server.exe"
	}
	return "server"
}

func startBackend(root, logDir, tunnelURL string) *proc {
	cmd := exec.Command(filepath.Join(logDir, serverName()))
	cmd.Dir = filepath.Join(root, "backend")
	// The backend serves the built bundle, so the tunnel reaches one port.
	// Listen on every interface so other devices on the LAN can connect.
	cmd.Env = append(os.Environ(),
		"WEB_ROOT="+filepath.Join(root, "frontend", "dist"),
		"LISTEN_ADDR=0.0.0.0:"+backendPort,
	)
	if tunnelURL != "" {
		cmd.Env = append(cmd.Env, "PUBLIC_BASE_URL="+tunnelURL)
	}
	p := &proc{name: "backend", cmd: cmd, logPath: filepath.Join(logDir, "backend.log")}
	attachLog(p)
	return p
}

func attachLog(p *proc) {
	f, err := os.OpenFile(p.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fatal("open log %s: %v", p.logPath, err)
	}
	p.cmd.Stdout = f
	p.cmd.Stderr = f
	if err := p.cmd.Start(); err != nil {
		fatal("start %s: %v", p.name, err)
	}
}

func waitFor(url string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		res, err := client.Get(url)
		if err == nil {
			res.Body.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "  ! %s did not become ready in time\n", url)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "  ! could not open the browser: %v\n", err)
	}
}

// shutdown stops every child process (and its children).
func shutdown(procs []*proc) {
	for _, p := range procs {
		if p == nil || p.cmd == nil || p.cmd.Process == nil {
			continue
		}
		pid := p.cmd.Process.Pid
		if runtime.GOOS == "windows" {
			exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(pid)).Run()
			continue
		}
		// Kill the whole group so npm's node child dies too.
		exec.Command("pkill", "-P", fmt.Sprint(pid)).Run()
		p.cmd.Process.Signal(syscall.SIGTERM)
	}
}

// repoRoot finds the project root (the folder holding backend/go.mod).
// An explicit path argument wins, so the launcher can be installed globally.
func repoRoot() (string, error) {
	if len(os.Args) > 1 {
		dir, err := filepath.Abs(os.Args[1])
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(dir, "backend", "go.mod")); err == nil {
			return dir, nil
		}
		return "", fmt.Errorf("backend/go.mod not found in %s", dir)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "backend", "go.mod")); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", fmt.Errorf("backend/go.mod not found")
}

func logPath(root, name string) string {
	return filepath.Join(root, ".launcher", name+".log")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
