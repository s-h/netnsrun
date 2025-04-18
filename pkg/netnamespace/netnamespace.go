package netnamespace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type NetNameSpace struct {
	Inode   string   `json:"inode"`
	Pid     string   `json:"pid"`
	CmdLine []string `json:"cmdline"`
}

type NetNameSpaceCollection struct {
	NetNameSpaces map[string]*NetNameSpace
}

// 检查是否为有效的 PID 目录
func isPidDir(name string) bool {
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// 获取网络命名空间inode
func getNetNsInode(pid string) (string, error) {
	netNsPath := filepath.Join("/proc", pid, "ns/net")
	link, err := os.Readlink(netNsPath)
	if err != nil {
		return "", err
	}
	// 解析符号连接格式 net:[4026541992]
	parts := strings.SplitN(link, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid netns link format")
	}
	inode := strings.Trim(parts[1], "[]")
	return inode, nil
}

// 获取cmdline
func getNetNsCmdLine(pid string) ([]string, error) {
	netCmdLinePath := filepath.Join("/proc", pid, "cmdline")
	cmdline, err := os.ReadFile(netCmdLinePath)
	if err != nil {
		return nil, err
	}

	var args []string
	parts := bytes.Split(cmdline, []byte{0})
	for _, part := range parts {
		if len(part) > 0 {
			args = append(args, string(part))
		}
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("cmdline内容为空")
	}
	return args, nil

}

// 获取网络命名空间inode、pid信息
func GetNetNs() (*NetNameSpaceCollection, error) {
	nc := &NetNameSpaceCollection{
		NetNameSpaces: make(map[string]*NetNameSpace),
	}
	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", procDir, err)
		return nc, err
	}

	for _, entry := range entries {
		var nsnetwork = NetNameSpace{}
		// 获取pid
		pidStr := entry.Name()
		// fmt.Printf("%s\n", pidStr)
		if !isPidDir(pidStr) {
			continue
		}
		nsnetwork.Pid = pidStr
		// 获取inode
		inode, err := getNetNsInode(pidStr)
		if err != nil {
			fmt.Errorf("pid %s, inode get error", pidStr)
			return nc, err
		}
		nsnetwork.Inode = inode
		//获取cmdline
		cmdline, err := getNetNsCmdLine(pidStr)
		if err != nil {
			cmdline = nil
		}
		nsnetwork.CmdLine = cmdline
		// inode不存在则更新
		_, exists := nc.NetNameSpaces[inode]
		if !exists {
			nc.NetNameSpaces[inode] = &nsnetwork
			continue
		}
		// pid为1的进行更新
		if pidStr == "1" {
			nc.NetNameSpaces[inode] = &nsnetwork
			continue
		}
		// pasue容器进程则更新对应inode 不覆盖pid为1的
		if cmdline != nil && nc.NetNameSpaces[inode].Pid != "1" {
			if cmdline[0] == "/pause" {
				nc.NetNameSpaces[inode] = &nsnetwork
			}
		}
	}

	return nc, nil

}
