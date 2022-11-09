package main

import (
	"encoding/json"
	"fmt"
	"github.com/derekhjray/namespace/types"
	"os"
	"syscall"
)

func stat(filename string) {
	st, err := os.Stat(filename)
	if err != nil {
		Errorf("%v", err)
		os.Exit(1)
	}

	fi := &types.FileInfo{
		Name:       filename,
		Size:       st.Size(),
		Mode:       int64(st.Mode()),
		Perm:       st.Mode().Perm().String(),
		ModifyTime: st.ModTime().UnixNano(),
	}

	if sysStat, ok := st.Sys().(*syscall.Stat_t); ok && sysStat != nil {
		fi.Inode = int64(sysStat.Ino)
		fi.Uid = int(sysStat.Uid)
		fi.Gid = int(sysStat.Gid)
		fi.AccessTime = sysStat.Atim.Nano()
		fi.BlockSize = sysStat.Blksize
		fi.Blocks = sysStat.Blocks
		fi.Links = int64(sysStat.Nlink)
	}

	data, err := json.Marshal(fi)
	if err != nil {
		Errorf("Marshal file %s info result failed, %v", filename, err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}
