package main

import (
	"github.com/urfave/cli"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"httpDistri/smux"
	"strings"
	"sync"
	"time"
)
var (
	VERSION  = "SELFBUILD"
	devmap  devMap
)

type devMap struct {
	devs map[string]*device
	lock sync.Mutex
}

type device struct {
	Name string
	Host string
}
func HttpProxy(devmap *devMap) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		host := "127.0.0.1:8080"
		path := req.URL.Path
		if(strings.HasPrefix(path,"/dev/")){
			subpath := strings.TrimPrefix(path, "/dev/")
			if idx := strings.Index(subpath, "/"); idx>0{
				devHash := subpath[:idx]
				devmap.lock.Lock()
				if device,ok := devmap.devs[devHash];ok{
					path = subpath[idx:]
					host = device.Host
				}
				devmap.lock.Unlock()
			}
		}
		req.URL.Path = path
		req.URL.Host = host
		req.URL.Scheme = "http"
		req.Host = host
	}
	return &httputil.ReverseProxy{Director: director}
}

// handle multiplex-ed connection
func handleMux(conn io.ReadWriteCloser, config *Config) {
	// stream multiplex
	smuxConfig := smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = config.SockBuf
	smuxConfig.KeepAliveInterval = time.Duration(config.KeepAlive) * time.Second
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	devName := ""
	if err != nil {
		conn.Close()
		return
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		conn.Close()
		return
	}
	log.Println("listening: ",listener.Addr().(*net.TCPAddr).Port)

	mux, err := smux.Client(conn, smuxConfig)
	if err != nil {
		log.Println(err)
		return
	}
	mux.HandleCtrlMsg = func(msg *smux.CtrlMsg) {
		switch msg.Cmd {
		case "login":
			devName = msg.Data
			devmap.lock.Lock()
			if _,ok := devmap.devs[devName];!ok{
				host := listener.Addr().(*net.TCPAddr).String()
				dev := device{devName, host}
				devmap.devs[devName] = &dev
				log.Println("dev login:", devName, host)
			}else{
				log.Println("dev already login: ", devName)
			}
			devmap.lock.Unlock()
		}
	}
	mux.HandleCtrlErr = func() {
		listener.Close()
	}
	defer mux.Close()
	for {
		p1, err := listener.AcceptTCP()
		if err != nil {
			if p1 != nil{
				p1.Close()
			}
			log.Println(err)
			break
		}
		go handleClient(mux, p1, true)
	}
	if devName != "" {
		devmap.lock.Lock()
		delete(devmap.devs, devName)
		devmap.lock.Unlock()
	}
}

func handleClient(sess *smux.Session, p1 io.ReadWriteCloser, quiet bool) {
	if !quiet {
		log.Println("stream opened")
		defer log.Println("stream closed")
	}

	defer p1.Close()
	p2, err := sess.OpenStream()
	if err != nil {
		return
	}
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
	devmap.devs = make(map[string]*device)
	devmap.devs["dev0"] = &device{Name: "dev0", Host:"127.0.0.1",}
	devmap.devs["dev1"] = &device{Name: "dev1", Host:"127.0.0.1",}
	devmap.devs["dev2"] = &device{Name: "dev2", Host:"127.0.0.1",}
	devmap.devs["dev3"] = &device{Name: "dev3", Host:"127.0.0.1",}
	devmap.devs["dev4"] = &device{Name: "dev4", Host:"127.0.0.1",}
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	myApp := cli.NewApp()
	myApp.Name = "server"
	myApp.Usage = "server"
	myApp.Version = VERSION
	myApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "listen,l",
			Value: ":29900",
			Usage: "local listen address",
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
		config.Listen = c.String("listen")
		config.SockBuf = c.Int("sockbuf")
		config.KeepAlive = c.Int("keepalive")

		if c.String("c") != "" {
			err := parseJSONConfig(&config, c.String("c"))
			checkError(err)
		}

		log.Println("version:", VERSION)
		addr, err := net.ResolveTCPAddr("tcp", config.Listen)
		checkError(err)
		listener, err := net.ListenTCP("tcp", addr)
		checkError(err)

		log.Println("listening on:", listener.Addr())
		for {
			p1, err := listener.AcceptTCP()
			if err != nil {
				log.Fatalln(err)
			}
			checkError(err)
			go handleMux(p1, &config)
		}
	}
	go myApp.Run(os.Args)
	go unixSockServer()
	proxy := HttpProxy(&devmap)
	http.Handle("/",proxy)
	http.Handle("/ws/", WsProxy(&devmap))
	log.Fatal(http.ListenAndServe(":80", nil))
}
