# Namespace
A namespace executor help library, used to execute task in specified linux namespaces

# Example

```go
package main

import (
	"bytes"
	"log"
	"os/exec"
	"context"
	"github.com/derekhjray/namespace"
)

func ipLink(_ ...interface{}) error {
	var (
		stdout bytes.Buffer
	)

	cmd := exec.CommandContext(context.TODO(), "ip", "link")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil
	}

	log.Println(stdout.String())
	return nil
}

func main() {
	ns, err := namespace.New(namespace.Types(NET), namespace.Pid(2398))
	if err != nil {
		log.Println(err)
		return
	}

	err = ns.Execute(ipLink)
	if err != nil {
		log.Println(err)
		return
	}

	if err = ipLink(); err != nil {
		log.Println(err)
	}
}
```
