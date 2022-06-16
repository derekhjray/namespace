package namespace

import (
	"bytes"
	"context"
	"io/ioutil"
	"os/exec"
	"testing"
)

func TestExecute(t *testing.T) {
	iplink := func(...interface{}) error {
		var (
			stdout bytes.Buffer
		)

		cmd := exec.CommandContext(context.TODO(), "ip", "addr")
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return nil
		}

		t.Log(stdout.String())
		return nil
	}

	ns, err := New(Types(USER, UTS, NET), Pid(3220))
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

func TestCGO(t *testing.T) {
	ns, err := New(Pid(3220), Types(NET, MNT))
	if err != nil {
		t.Error(err)
		return
	}

	err = ns.Execute("cat", "/etc/passwd")
	if err != nil {
		t.Error(err)
		return
	}

	data, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		t.Error(err)
		return
	}

	t.Log(string(data))
}

func TestCat(t *testing.T) {
	buffers, err := Cat([]string{"/usr/local/tomcat/conf/tomcat-users.xml", "/etc/passwd"}, 89541)
	if err != nil {
		t.Error(err)
		return
	}

	for _, buffer := range buffers {
		t.Log(buffer.String())
	}
}
