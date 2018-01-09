// +build daemon

package main

import (
	"log"

	"github.com/docker/docker/builtins"
	"github.com/docker/docker/daemon"
	_ "github.com/docker/docker/daemon/execdriver/lxc"
	_ "github.com/docker/docker/daemon/execdriver/native"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
)

const CanDaemon = true

var (
	daemonCfg = &daemon.Config{}
)

func init() {
	daemonCfg.InstallFlags()
}

//这是Docker Daemon的主要运行函数
func mainDaemon() {
	if flag.NArg() != 0 {
		flag.Usage()
		return
	}
	//创建engine对象
	// 更新engine的一些属性  主要是handlers的map保存了命令和handler的映射关系
	eng := engine.New()
	//设置engine的信号捕获以及处理方法
	signal.Trap(eng.Shutdown)
	// Load builtins
	if err := builtins.Register(eng); err != nil {
		log.Fatal(err)
	}

	// load the daemon in the background so we can immediately start
	// the http api so that connections don't fail while the daemon
	// is booting
	// 这边通过gorountine的方式先加载daemon对象并运行，保证在之后的的booting过程中不影响到connection的建立
	go func() {
		//创建daemon对象
		d, err := daemon.NewDaemon(daemonCfg, eng)
		if err != nil {
			log.Fatal(err)
		}
		//向engine对象注册更多的处理方法
		if err := d.Install(eng); err != nil {
			log.Fatal(err)
		}
		// after the daemon is done setting up we can tell the api to start
		// accepting connections
		// 开始运行acceptConnections的job，即往init进程发送READY=1，告知apiServer可以开始接收请求了
		if err := eng.Job("acceptconnections").Run(); err != nil {
			log.Fatal(err)
		}
	}()
	// TODO actually have a resolved graphdriver to show?
	log.Printf("docker daemon: %s %s; execdriver: %s; graphdriver: %s",
		dockerversion.VERSION,
		dockerversion.GITCOMMIT,
		daemonCfg.ExecDriver,
		daemonCfg.GraphDriver,
	)

	// Serve api
	//启动serveapi并设置env
	job := eng.Job("serveapi", flHosts...)
	job.SetenvBool("Logging", true)
	job.SetenvBool("EnableCors", *flEnableCors)
	job.Setenv("Version", dockerversion.VERSION)
	job.Setenv("SocketGroup", *flSocketGroup)

	job.SetenvBool("Tls", *flTls)
	job.SetenvBool("TlsVerify", *flTlsVerify)
	job.Setenv("TlsCa", *flCa)
	job.Setenv("TlsCert", *flCert)
	job.Setenv("TlsKey", *flKey)
	job.SetenvBool("BufferRequests", true)
	if err := job.Run(); err != nil {
		log.Fatal(err)
	}
}
