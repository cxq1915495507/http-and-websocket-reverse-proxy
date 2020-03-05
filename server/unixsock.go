package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
	"syscall"
)

type UnixMsg struct {
	Cmd  string
	Data string
}

const (
	sockpath = "/dev/shm/unixsock"
)
func EncodeUnixMsg(cmd string, data string) []byte {
	mess := UnixMsg{cmd, data}
	jsonMess, err := json.Marshal(mess)
	if err != nil {
		log.Println(err)
	}
	return append(jsonMess, '\n')
}

func DecodeUnixMsg(str string) *UnixMsg {
	var mess = new(UnixMsg)
	err := json.Unmarshal([]byte(str), &mess)
	if err != nil {
		return nil
	}
	return mess
}

func EncodeDevList() string {
	//var devs []device
	//for devname:= range devmap.devs{
	//	devs = append(devs, *devmap.devs[devname])
	//}
	//log.Println(devs)
	jsonMess, err := json.Marshal(devmap.devs)
	if err != nil {
		log.Println(err)
	}
	return string(jsonMess)
}

func unixPipe(conn *net.UnixConn) {
	ipStr := conn.RemoteAddr().String()
	retMsg := []byte{}
	defer func() {
		log.Println("disconnected :" + ipStr)
		conn.Close()
	}()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		log.Println("msg:",line)
		msg := DecodeUnixMsg(line)
		if msg == nil{
			log.Println("DecodeUnixMsg err")
			continue
		}
		devmap.lock.Lock()
		log.Println("Cmd:",msg.Cmd)
		switch msg.Cmd{
		case "devHost":
			name := msg.Data
			host := ""
			log.Println("Name:",name)
			if dev,ok := devmap.devs[name];ok{
				host = dev.Host
			}else{
				host = "null"
			}
			retMsg = EncodeUnixMsg("devHost", host)
		case "devList":
			jsonMess, err := json.Marshal(devmap.devs)
			if err != nil {
				log.Println(err)
			}
			retMsg = EncodeUnixMsg("devList", string(jsonMess))
		default:
			retMsg = EncodeUnixMsg("unknow", "null")
		}
		devmap.lock.Unlock()
		conn.Write(retMsg)
	}
}

func unixSockServer() {
	var unixAddr *net.UnixAddr

	syscall.Unlink(sockpath)
	unixAddr, _ = net.ResolveUnixAddr("unix", sockpath)

	unixListener, _ := net.ListenUnix("unix", unixAddr)
	os.Chmod(sockpath, 0777)
	defer unixListener.Close()

	for {
		unixConn, err := unixListener.AcceptUnix()
		if err != nil {
			continue
		}

		log.Println("A client connected : " + unixConn.RemoteAddr().String())
		go unixPipe(unixConn)
	}
}