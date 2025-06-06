package main

import (
	"fmt"
	"net"

	"msh/lib/config"
	"msh/lib/conn"
	"msh/lib/errco"
	"msh/lib/input"
	"msh/lib/progmgr"
	"msh/lib/servctrl"
	"msh/lib/utility"
)

// contains intro to script and program
var intro []string = []string{
	"              | |        _   ",
	" _ __ ___  ___| |__    _| |_ ",
	"| '_ ` _ \\/ __| '_ \\  |_   _|",
	"| | | | | \\__ \\ | | |   |_|",
	"|_| |_| |_|___/_| |_|        ",
	"      " + progmgr.MshVersion + func() string {
		if progmgr.IsPrerelease {
			return " " + progmgr.PrereleaseSuffix
		} else {
			return ""
		}
	}(),
	"      " + progmgr.MshCommit,
	"forked by: kinuseka  (by gekigek99)",
	"github: https://github.com/kinuseka",
}

func main() {
	// print program intro
	// not using errco.NewLogln since log time is not needed
	fmt.Println(utility.Boxify(intro))

	// load configuration from msh config file
	logMsh := config.LoadConfig()
	if logMsh != nil {
		logMsh.Log(true)
		progmgr.AutoTerminate()
	}

	// launch msh manager
	go progmgr.MshMgr()
	// wait for the initial update check
	<-progmgr.ReqSent

	// if ms suspension is allowed, pre-warm the server
	if config.ConfigRuntime.Msh.SuspendAllow {
		errco.NewLogln(errco.TYPE_INF, errco.LVL_1, errco.ERROR_NIL, "minecraft server will now pre-warm (SuspendAllow is enabled)...")
		logMsh = servctrl.WarmMS()
		if logMsh != nil {
			logMsh.Log(true)
		}
	}

	// launch GetInput()
	go input.GetInput()

	// ---------------- connections ---------------- //

	// launch query handler
	if config.ConfigRuntime.Msh.EnableQuery {
		go conn.HandlerQuery()
	}

	// open a tcp listener
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.MshHost, config.MshPort))
	if err != nil {
		errco.NewLogln(errco.TYPE_ERR, errco.LVL_3, errco.ERROR_CLIENT_LISTEN, err.Error())
		progmgr.AutoTerminate()
	}

	// infinite cycle to handle new clients.
	errco.NewLogln(errco.TYPE_INF, errco.LVL_1, errco.ERROR_NIL, "%-40s %10s:%5d ...", "listening for new clients connections on", config.MshHost, config.MshPort)
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			errco.NewLogln(errco.TYPE_ERR, errco.LVL_3, errco.ERROR_CLIENT_ACCEPT, err.Error())
			continue
		}

		go conn.HandlerClientConn(clientConn)
	}
}
