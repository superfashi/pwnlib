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
	line, _ := cmd.ReadLine()
	if !bytes.Equal(line, []byte("Hello world")) {
		t.Fatal("process output mismatch")
	}
	line, _ = cmd.ReadUntil(false, []byte{','})
	if !bytes.Equal(line, []byte("Wow,")) {
		t.Fatal("process output mismatch")
	}
	line, _ = cmd.ReadRegex(regexp.MustCompile(`.*data`), false)
	if !bytes.Equal(line, []byte(" such data")) {
		t.Fatal("process output mismatch")
	}
	line, _ = cmd.ReadLine()
	if len(line) > 0 {
		t.Fatal("process output mismatch")
	}
}
