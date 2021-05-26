package tubes

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"
)

func TestPython(t *testing.T) {
	command := exec.Command("python")
	cmd, err := FromCmd(command)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Close()
	}()
	_, err = cmd.SendLine([]byte("print('Hello world')"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = cmd.SendLine([]byte("print('Wow, such data')"))
	if err != nil {
		t.Fatal(err)
	}
	err = cmd.Shutdown(Send)
	if err != nil {
		t.Fatal(err)
	}
	line, _ := cmd.RecvLine()
	if !bytes.Equal(line, []byte("Hello world")) {
		t.Fatal("process output mismatch")
	}
	line, _ = cmd.RecvUntil(false, ',')
	if !bytes.Equal(line, []byte("Wow,")) {
		t.Fatal("process output mismatch")
	}
	line, _ = cmd.RecvRegex(regexp.MustCompile(`.*data`), false)
	if !bytes.Equal(line, []byte(" such data")) {
		t.Fatal("process output mismatch")
	}
	line, _ = cmd.Recv(2)
	if !bytes.Equal(line, cmd.Newline) {
		t.Fatal("process output mismatch")
	}
}
