package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/haproxytech/client-native/v2/runtime"
)

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

func writeBaseConfig(config *Config) error {
	fp, err := os.Create(config.HaproxyConfPath)
	if err != nil {
		return err
	}
	defer fp.Close()
	return haproxyConf.Execute(fp, config)
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	if err := sentry.Init(sentry.ClientOptions{
		Debug: true,
	}); err != nil {
		log.Fatal(err)
	}
	defer sentry.Flush(5 * time.Second)
	defer sentryRecoverRepanic()

	config, err := getConfig()
	maybePanic(err)

	maybePanic(writeBaseConfig(config))

	exit := make(chan struct{})

	// Intercept our shutdown signals so we can cleanup nicely
	signalHandler := make(chan os.Signal)
	signal.Notify(signalHandler, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer sentryRecoverRepanic()
		<-signalHandler
		exit <- struct{}{}
	}()

	firstTick := true

	client := runtime.SingleRuntime{}

	d := NewDiscovery(config)
	d.Loop(func(hosts []Host) {
		if firstTick {
			// On our first tick, we want to spawn haproxy subprocess
			firstTick = false

			haproxy := exec.Command(config.HaproxyBinPath, "-W", "-db", "-f", config.HaproxyConfPath)
			haproxy.Stdout = os.Stdout
			haproxy.Stderr = os.Stderr
			maybePanic(haproxy.Start())

			go func() {
				defer sentryRecoverRepanic()
				haproxy.Wait()
				exit <- struct{}{}
			}()

			client.Init(config.HaproxyAdminSocket, 0, 0)

			// wait for our haproxy admin socket to become available
			for {
				time.Sleep(10 * time.Millisecond)
				if err := client.Execute("help"); err == nil {
					break
				}
			}

		}

		// We guarantee we get a Host entry for each haproxy slot
		// so we buffer up a command that represents the state of each slot
		// and write it all to haproxy at once.
		var buffer strings.Builder
		for i := 0; i < config.HaproxySlots; i++ {
			host := hosts[i]
			log.Println(i, host)
			if host.IsEmpty() {
				// Down nodes are considered in "maintenance" mode
				buffer.WriteString(fmt.Sprintf("set server upstream/be%d state maint;", i))
			} else {
				buffer.WriteString(fmt.Sprintf("set server upstream/be%d addr %s port %d;", i, hosts[i].IP, hosts[i].Port))
				buffer.WriteString(fmt.Sprintf("set server upstream/be%d state ready;", i))
			}
		}

		client.ExecuteRaw(buffer.String())
	})

	<-exit

	// attempt to clean up our files we wrote to disk to be nice
	os.Remove(config.HaproxyAdminSocket)
	os.Remove(config.HaproxyConfPath)
}

// sentryRecoverRepanic recovers from a runtime panic, reports it to Sentry and
// starts panicking again.
func sentryRecoverRepanic() {
	if err := recover(); err != nil {
		sentry.CurrentHub().Recover(err)
		panic(err)
	}
}
