package main

import (
        "github.com/urfave/cli"
        "io"
        "log"
        "net"
        "os"
        "http-and-websocket-reverse-proxy/smux"
        "time"
)
var (
        VERSION = "SELFBUILD"
)

func handleClient(p1, p2 io.ReadWriteCloser, quiet bool) {
        if !quiet {
                log.Println("stream opened")
                defer log.Println("stream closed")
        }
        defer p1.Close()
        defer p2.Close()

        // start tunnel
        p1die := make(chan struct{})
        buf1 := make([]byte, 65535)
        go func() { io.CopyBuffer(p1, p2, buf1); close(p1die) }()

        p2die := make(chan struct{})
        buf2 := make([]byte, 65535)
        go func() { io.CopyBuffer(p2, p1, buf2); close(p2die) }()

        // wait for tunnel termination
        select {
        case <-p1die:
        case <-p2die:
        }
}

func checkError(err error) {
        if err != nil {
                log.Printf("%+v\n", err)
                os.Exit(-1)
        }
}


func main() {
        log.SetFlags(log.LstdFlags | log.Lshortfile)
        myApp := cli.NewApp()
        myApp.Name = "client"
        myApp.Usage = "client"
        myApp.Version = VERSION
        myApp.Flags = []cli.Flag{
                cli.StringFlag{
                        Name:  "target,t",
                        Value: "127.0.0.1:80",
                        Usage: "local listen address",
                },
                cli.StringFlag{
                        Name:  "remoteaddr, r",
                        Value: "127.0.0.1:29900",
                        Usage: "kcp server address",
                },
                cli.StringFlag{
                        Name:   "key",
                        Value:  "abc",
                        Usage:  "pre-shared secret between client and server",
                        EnvVar: "KCPTUN_KEY",
                },
                cli.IntFlag{
                        Name:  "sockbuf",
                        Value: 4194304, // socket buffer size in bytes
                        Usage: "per-socket buffer in bytes",
                },
                cli.IntFlag{
                        Name:  "keepalive",
                        Value: 10, // nat keepalive interval in seconds
                        Usage: "seconds between heartbeats",
                },
        }
        myApp.Action = func(c *cli.Context) error {
                config := Config{}
                config.Target = c.String("target")
                config.RemoteAddr = c.String("remoteaddr")
                config.Key = c.String("key")
                config.SockBuf = c.Int("sockbuf")
                config.KeepAlive = c.Int("keepalive")

                if c.String("c") != "" {
                        err := parseJSONConfig(&config, c.String("c"))
                        checkError(err)
                }

                log.Println("version:", VERSION)
                log.Println("remote address:", config.RemoteAddr)
                log.Println("target address:", config.Target)
                smuxConfig := smux.DefaultConfig()
                smuxConfig.MaxReceiveBuffer = config.SockBuf
                smuxConfig.KeepAliveInterval = time.Duration(config.KeepAlive) * time.Second
                for {
                        conn, err := net.DialTimeout("tcp", config.RemoteAddr, 5*time.Second)
                        // stream multiplex
                        if err != nil {
                                log.Println(err)
                                time.Sleep(10*time.Second)
                                continue
                        }
                        mux, err := smux.Server(conn, smuxConfig)
                        if err != nil {
                                log.Println(err)
                                time.Sleep(10*time.Second)
                                continue
                        }

                        mux.WriteCtrlStream(smux.EncodeCtrlMsg("login",config.Key))
                        mux.HandleCtrlMsg = func(msg *smux.CtrlMsg) {
                                switch msg.Cmd {
                                case "fin":
                                        log.Println("get fin")
                                        os.Exit(-1)
                                }
                        }
                        log.Println("connection:", conn.LocalAddr(), "->", conn.RemoteAddr())

                        for {
                                p1, err := mux.AcceptStream()
                                if err != nil {
                                        log.Println(err)
                                        break
                                }
                                p2, err := net.DialTimeout("tcp", config.Target, 5*time.Second)
                                if err != nil {
                                        p1.Close()
                                        log.Println(err)
                                        continue
                                }
                                go handleClient(p1, p2, true)
                        }
                }
        }
        myApp.Run(os.Args)
}
