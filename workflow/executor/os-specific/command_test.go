package os_specific

import (
	"bytes"
	"io"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSimpleStartCloser(t *testing.T) {
	var cmd *exec.Cmd
	var expectedOutput string

	if runtime.GOOS == "windows" {
		// Use PowerShell on Windows to write output without newline
		cmd = exec.Command("powershell", "-Command", `Write-Host -NoNewline "A123456789B123456789C123456789D123456789E123456789"`)
		expectedOutput = "A123456789B123456789C123456789D123456789E123456789"
	} else {
		// Use shell echo with \c flag on Unix systems
		cmd = exec.Command("sh", "-c", `echo "A123456789B123456789C123456789D123456789E123456789\c"`)
		expectedOutput = "A123456789B123456789C123456789D123456789E123456789"
	}

	var stdoutWriter bytes.Buffer
	slowWriter := SlowWriter{
		&stdoutWriter,
	}
	// Command outputs are asynchronously written to cmd.Stdout.
	// Using SlowWriter causes the situation where the invoked command has exited but its outputs have not been written yet.
	cmd.Stdout = slowWriter

	closer, err := StartCommand(cmd)
	assert.NoError(t, err)
	err = cmd.Process.Release()
	assert.NoError(t, err)
	// Wait for echo command to exit before calling closer
	time.Sleep(100 * time.Millisecond)
	closer()

	assert.Equal(t, expectedOutput, stdoutWriter.String())
}

type SlowWriter struct {
	Writer io.Writer
}

func (s SlowWriter) Write(data []byte) (n int, err error) {
	for i := range data {
		_, err := s.Writer.Write(data[i : i+1])
		if err != nil {
			return i, err
		}
		time.Sleep(7 * time.Millisecond)
	}
	return len(data), nil
}
