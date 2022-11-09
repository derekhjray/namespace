package types

type FileInfo struct {
	Name       string
	Perm       string
	Uid        int
	Gid        int
	Size       int64
	Mode       int64
	Inode      int64
	BlockSize  int64
	Blocks     int64
	Links      int64
	AccessTime int64
	ModifyTime int64
}
