package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/s-h/netnsrun/pkg/netnamespace"

	"golang.org/x/sys/unix"
)

type CmdLine struct {
	Name string   `json:"inode"`
	Args []string `json:"args"`
}

// 获取命令参数
func getArgs() (CmdLine, error) {
	var cmdline CmdLine
	if len(os.Args) != 2 {
		return cmdline, fmt.Errorf("输入要执行的命令, 使用\"进行引用，例如：./main.go \"ip add\"")
	}
	fmt.Println(os.Args[1])
	parts := strings.Split(os.Args[1], " ")
	cmdline.Name = parts[0]
	if len(parts) > 1 {
		cmdline.Args = parts[1:]
	}
	return cmdline, nil
}

// 子进程执行逻辑（需单独编译为二进制文件）
func childProcess() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 从环境变量获取文件描述符
	fdStr := os.Getenv("NAMESPACE_FD")
	fd, _ := strconv.Atoi(fdStr)
	// 从环境变量获取原始pid
	oPid := os.Getenv("ORIGIN_PID")
	inode := os.Getenv("INODE")

	// 进入目标网络命名空间
	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		panic(err)
	}
	// 从环境变量获取 cmdline 并反序列化
	cmdlineJSON := os.Getenv("CMDLINE")
	var cmdline CmdLine
	if err := json.Unmarshal([]byte(cmdlineJSON), &cmdline); err != nil {
		panic("反序列化失败: " + err.Error())
	}
	// 在此执行需要在目标命名空间运行的命令
	fmt.Printf("\n>>在[%s]命名空间中执行命令，原始PID为: %s\n", inode, oPid)
	cmd := exec.Command(cmdline.Name, cmdline.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// 父进程控制逻辑，指定命名空间运行程序
func NetNameSpaceRun() error {
	cmdline, err := getArgs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// 将 cmdline 序列化为 JSON
	cmdlineJSON, err := json.Marshal(cmdline)
	if err != nil {
		fmt.Println("序列化失败:", err)
		return err
	}
	// 示例：要遍历的PID列表（替换为实际需要操作的PID）
	netns, err := netnamespace.GetNetNs()
	if err != nil {
		fmt.Println(err)
		return err
	}

	for i, _ := range netns.NetNameSpaces {
		// 打开目标命名空间文件
		pid := netns.NetNameSpaces[i].Pid
		inode := netns.NetNameSpaces[i].Inode
		nsPath := fmt.Sprintf("/proc/%s/ns/net", pid)
		nsFile, err := os.Open(nsPath)
		if err != nil {
			fmt.Printf("无法打开命名空间文件 %s: %v\n", nsPath, err)
			continue
		}
		defer nsFile.Close()

		// 启动子进程
		cmd := exec.Command(os.Args[0], "-child") // 假设本程序支持-child参数进入子模式
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("NAMESPACE_FD=%d", 3), // 文件描述符从3开始
			"GO_CHILD_MODE=1",
			fmt.Sprintf("CMDLINE=%s", cmdlineJSON), // 传递 cmdline
			fmt.Sprintf("ORIGIN_PID=%s", pid),      //传递原始pid
			fmt.Sprintf("INODE=%s", inode),         //传递原始inode
		)
		cmd.ExtraFiles = []*os.File{nsFile} // 传递文件描述符给子进程
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("执行失败(PID %d): %v\n", pid, err)
		}
	}
	return nil
}

// 程序入口
func init() {
	if os.Getenv("GO_CHILD_MODE") == "1" {
		childProcess()
		os.Exit(0)
	}
}

func main() {
	NetNameSpaceRun()
}
