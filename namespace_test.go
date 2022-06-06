package namespace

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
)

func TestExecute(t *testing.T) {
	iplink := func(...interface{}) error {
		var (
			stdout bytes.Buffer
		)

		cmd := exec.CommandContext(context.TODO(), "ip", "link")
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return nil
		}

		t.Log(stdout.String())
		return nil
	}

	ns, err := New(Types(NET), Pid(2398))
	if err != nil {
		t.Error(err)
		return
	}

	err = ns.Execute(iplink)
	if err != nil {
		t.Error(err)
		return
	}

	if err = iplink(); err != nil {
		t.Error(err)
	}
}
