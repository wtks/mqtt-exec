package main

import (
	"context"
	"github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"sync"
)

type Entry struct {
	Name             string `yaml:"-"`
	Topic            string
	Command          string
	Args             []string
	WorkingDirectory string
	MultipleInstance bool
	Qos              *byte

	mutex   sync.Mutex `yaml:"-"`
	running bool       `yaml:"-"`
}

func (entry *Entry) receiveMessage(client mqtt.Client, message mqtt.Message) {
	entry.execCommand(context.Background())
}

func (entry *Entry) execCommand(ctx context.Context) {
	log := log.WithField("entry", entry.Name)

	entry.mutex.Lock()
	if !entry.MultipleInstance && entry.running {
		return
	}
	entry.running = true
	entry.mutex.Unlock()
	defer func() {
		entry.mutex.Lock()
		entry.running = false
		entry.mutex.Unlock()
	}()

	cmd := exec.Command(entry.Command, entry.Args...)
	cmd.Dir = entry.WorkingDirectory

	log.Info("command starts...")
	result, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).Error("execution failed")
		return
	}
	log.Print(string(result))
	log.Info("command succeeded")
}
