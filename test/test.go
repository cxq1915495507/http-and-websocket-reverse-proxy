package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type UnixMsg struct {
Cmd  string
Data string
}

func EncodeUnixMsg(cmd string, data string) []byte {
mess := UnixMsg{cmd, data}
jsonMess, err := json.Marshal(mess)
if err != nil {
log.Println(err)
}
return append(jsonMess, '\n')
}

func main(){
	fmt.Println(string(EncodeUnixMsg("cmd", "data")))
}