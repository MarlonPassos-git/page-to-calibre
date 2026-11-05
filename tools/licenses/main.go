// licenses copies dependency notices from the exact Go module cache used to
// build the executable. It runs at build time, never in the application.
package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: licenses DEST")
	}
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err = cmd.Start(); err != nil {
		panic(err)
	}
	d := json.NewDecoder(pipe)
	for {
		var m struct {
			Path, Version, Dir string
			Main               bool
		}
		if err = d.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		if m.Main || m.Dir == "" {
			continue
		}
		entries, err := os.ReadDir(m.Dir)
		if err != nil {
			panic(err)
		}
		for _, entry := range entries {
			upper := strings.ToUpper(entry.Name())
			if entry.IsDir() || !(strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "LICENCE") || strings.HasPrefix(upper, "COPYING") || strings.HasPrefix(upper, "NOTICE") || strings.HasPrefix(upper, "COPYRIGHT")) {
				continue
			}
			content, err := os.ReadFile(filepath.Join(m.Dir, entry.Name()))
			if err != nil {
				panic(err)
			}
			dest := filepath.Join(os.Args[1], m.Path+"@"+m.Version, entry.Name())
			if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				panic(err)
			}
			if err = os.WriteFile(dest, content, 0644); err != nil {
				panic(err)
			}
		}
	}
	if err = cmd.Wait(); err != nil {
		panic(err)
	}
}
