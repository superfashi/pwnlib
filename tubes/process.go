package tubes

import (
	"io"
	"os"
	"os/exec"
	"time"
)

type cmdWrapper struct {
	inWrite io.WriteCloser
	outRead interface {
		io.ReadCloser
		SetReadDeadline(time.Time) error
	}
	cmd *exec.Cmd
}

func (c *cmdWrapper) Shutdown(direction TubeDirection) (err error) {
	switch direction {
	case Read:
		return c.outRead.Close()
	case Send:
		return c.inWrite.Close()
	}
	return nil
}

func (c *cmdWrapper) Read(p []byte) (n int, err error) {
	return c.outRead.Read(p)
}

func (c *cmdWrapper) Write(p []byte) (n int, err error) {
	return c.inWrite.Write(p)
}

func (c *cmdWrapper) Close() error {
	_ = c.cmd.Process.Kill()
	return c.cmd.Wait()
}

func (c *cmdWrapper) SetReadDeadline(t time.Time) error {
	return c.outRead.SetReadDeadline(t)
}

func FromCmd(cmd *exec.Cmd) (*Tube, error) {
	inPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	outPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = cmd.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return NewTube(&cmdWrapper{inPipe, outPipe.(*os.File), cmd}), nil
}
