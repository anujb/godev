package main

import (
	"flag"
	"fmt"
	"github.com/howeyc/fsnotify"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	baseDir string
	dir     string
	pid     int
)

func init() {

	pid = 0
	cwd, _ := os.Getwd()
	flag.StringVar(&dir, "dir", cwd, "Directory to watch")

	baseDir = filepath.Base(dir)

	flag.Parse()
}

func main() {

	fmt.Println("Starting Watcher")

	watcher, err := fsnotify.NewWatcher()

	if err != nil {
		log.Fatalf("Unable to watch: ", dir)
		log.Fatalf("With error: ", err)
	}

	startBuildAndRun()

	quitWatcher := make(chan bool)
	startWatcher(watcher, quitWatcher)

	watchRecursive(dir, watcher)
	log.Println("Watching directory: ", dir)

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt)

	select {
	case <-interrupt:

		log.Println("Shutting down watcher.")

		watcher.Close()
		quitWatcher <- true

		log.Println("Quit watcher.")
	}

}

func startWatcher(watcher *fsnotify.Watcher, quit chan bool) {
	go func() {
		sem := make(chan int, 1)

		for {
			select {
			case evt := <-watcher.Event:

				if evt.IsModify() {

					sem <- 1

					go func() {

						//if already running proc, kill it

						if pid > 0 {
							log.Println("Killing old process id", pid)
							err := syscall.Kill(pid, 9)

							if err != nil {
								log.Println("Unable to kill process!")
							}
						}

						// restart build and run steps

						startBuildAndRun()

						<-sem
					}()
				}

			case err := <-watcher.Error:
				log.Println(err)
			case <-quit:
				return

			}
		}
	}()

}

func watchRecursive(dir string, watcher *fsnotify.Watcher) int {
	var watched int

	entries, err := ioutil.ReadDir(dir)

	if err != nil {
		log.Println(err)
		return 0
	}

	watcher.Watch(dir)
	watched++

	for _, e := range entries {
		name := e.Name()

		if e.IsDir() && strings.HasPrefix(name, ".") == false {
			watched += watchRecursive(filepath.Join(dir, name), watcher)

		}
	}

	return watched
}

func startBuildAndRun() {

	log.Println("Building app.", baseDir)

	buildProc := exec.Command("go", "build")

	buildProc.Stderr = os.Stderr
	buildProc.Stdout = os.Stdout

	if err := buildProc.Run(); err != nil {
		log.Printf("Build %q failed, with error %v\n", buildProc.Path, err)
	}

	absolutePath := path.Join(dir, baseDir)
	startProc := exec.Command(absolutePath)

	startProc.Stderr = os.Stderr
	startProc.Stdout = os.Stdout

	log.Println("Starting app", baseDir)

	if err := startProc.Start(); err != nil {
		log.Printf("Command %q failed, with error %v\n", startProc.Path, err)
		pid = 0
	}

	pid = startProc.Process.Pid
}
