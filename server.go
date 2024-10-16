package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// 下线时间
const OFLTIME int64 = 120

type Server struct {
	Ip   string
	Port int
	// 在线用户的列表
	OnlineMap map[string]*User
	mapLock   sync.RWMutex
	// 消息广播的channel
	Message chan string
}

// 广播消息
func (this *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	this.Message <- sendMsg
}

func (this *Server) Handler(conn net.Conn) {
	// ...当前链接的业务
	fmt.Println("连接建立成功")
	user := NewUser(conn, this)
	// 用户上线  感觉耦合度有点高，作为server的成员函数，接收一个user指针应该会更好？
	user.Online()

	// 监听当前用户是否活跃的channel
	isLive := make(chan bool)

	// 接受客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				user.Offline()
				return
			}
			if err != nil && err != io.EOF {
				fmt.Println("Conn Read err:", err)
				return
			}
			// 提取用户消息(去除'\n)
			msg := string(buf[:n-1])
			// 用户对msg进行消息处理
			user.DoMessage(msg)
			// 用户任意消息，代表活跃
			isLive <- true
		}
	}()
	// 当前handler拥塞
	for {
		select {
		case <-isLive:
			// 当前用户活跃，应重置定时器。不做任何事
		case <-time.After(time.Second * time.Duration(OFLTIME)):
			// 超时
			// 将当前User强制关闭
			user.SendMsg("你被踢了")
			// 销毁用的资源
			close(user.C)
			// 关闭连接
			conn.Close()
			// 退出当前Handler
			return // 或者 runtime.Goexit()
		}
	}
}

// 创建一个server的接口
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}
	return server
}

// 监听Message广播消息channel的goroutine，一旦有消息就发送给全部的在线User
func (this *Server) ListenMessage() {
	for {
		msg := <-this.Message
		// 将msg发送给全部的在线User
		this.mapLock.Lock()
		for _, cli := range this.OnlineMap {
			cli.C <- msg
		}
		this.mapLock.Unlock()
	}
}

// 启动服务器的接口
func (this *Server) Start() {
	// socker listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
	if err != nil {
		fmt.Println("net.Listen err:", err)
		return
	}
	// close listener socket
	defer listener.Close()
	// 启动监听Message的goroutine
	go this.ListenMessage()

	for {
		// accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Listen accept err:", err)
			continue
		}
		// do handler
		go this.Handler(conn)
	}
}
