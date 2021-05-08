package tubes

import (
	"io"
	"os/exec"
	"time"
)

type cmdWrapper struct {
	inWrite *io.PipeWriter
	outRead *io.PipeReader
	cmd     *exec.Cmd
}

func (c *cmdWrapper) shutdown(direction TubeDirection) (err error) {
	if direction&Read != 0 {
		err = c.outRead.Close()
	}
	if direction&Send != 0 {
		if err2 := c.inWrite.Close(); err2 != nil {
			err = err2
		}
	}
	return
}

func (c *cmdWrapper) Read(p []byte) (n int, err error) {
	return c.outRead.Read(p)
}

func (c *cmdWrapper) Write(p []byte) (n int, err error) {
	return c.inWrite.Write(p)
}

func (c *cmdWrapper) Close() error {
	defer func() {
		_ = c.inWrite.Close()
		_ = c.outRead.Close()
	}()
	_ = c.cmd.Process.Kill()
	return c.cmd.Wait()
}

func (c *cmdWrapper) setReadTimeout(time.Duration) {}

func FromCmd(cmd *exec.Cmd) (*Tube, error) {
	inRead, inWrite := io.Pipe()
	outRead, outWrite := io.Pipe()
	cmd.Stdin = inRead
	cmd.Stdout = outWrite
	cmd.Stderr = outWrite
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return NewTube(&cmdWrapper{
		inWrite, outRead, cmd,
	}), nil
}
