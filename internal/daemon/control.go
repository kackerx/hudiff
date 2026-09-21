package daemon

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/kackerx/hudiff/internal/session"
)

type Info struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Started string `json:"started"`
}

func ReadInfo() Info {
	var i Info
	_ = session.ReadJSON(session.ServerFile(), &i)
	return i
}

// Alive matches the oracle: the recorded pid must exist and the port must
// accept a connection.
func (i Info) Alive() bool {
	if i.PID == 0 || i.Port == 0 {
		return false
	}
	p, err := os.FindProcess(i.PID)
	if err != nil || p.Signal(syscall.Signal(0)) != nil {
		return false
	}
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", i.Port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// Ensure returns the running daemon's port, spawning a detached one if needed.
func Ensure(wantPort int) (int, error) {
	if i := ReadInfo(); i.Alive() {
		return i.Port, nil
	}
	if err := os.MkdirAll(session.StateDir(), 0o755); err != nil {
		return 0, err
	}
	if err := spawn(wantPort); err != nil {
		return 0, err
	}
	for range 60 {
		time.Sleep(50 * time.Millisecond)
		if i := ReadInfo(); i.Alive() {
			return i.Port, nil
		}
	}
	return 0, fmt.Errorf("server 没起来, 看 %s/server.log", session.StateDir())
}

func spawn(wantPort int) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(session.StateDir()+"/server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	args := []string{"daemon", "serve"}
	if wantPort > 0 {
		args = append(args, "--port", fmt.Sprint(wantPort))
	}
	return startDetached(self, args, log)
}

func Stop() string {
	i := ReadInfo()
	if !i.Alive() {
		return "server 本来就没在跑"
	}
	if p, err := os.FindProcess(i.PID); err == nil {
		_ = p.Signal(syscall.SIGTERM)
	}
	return fmt.Sprintf("已停止 server(pid %d), 会话数据都在", i.PID)
}

func HomeURL(wantPort int) (string, error) {
	port, err := Ensure(wantPort)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("http://127.0.0.1:%d/", port), nil
}

func SessionURL(sid string, wantPort int) (string, error) {
	home, err := HomeURL(wantPort)
	if err != nil {
		return "", err
	}
	return home + "s/" + sid + "/", nil
}

func StatusLine() string {
	i := ReadInfo()
	if !i.Alive() {
		return "server: ○ 未运行(任何子命令会自动拉起)"
	}
	return fmt.Sprintf("server: ● http://127.0.0.1:%d/", i.Port)
}
