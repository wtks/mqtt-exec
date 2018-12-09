package main

import (
	"flag"
	"github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
)

var (
	mqttHost       string
	mqttClientId   string
	mqttUsername   string
	mqttPassword   string
	mqttDefaultQOS uint
	configYaml     string

	entries []*Entry
)

func init() {
	flag.StringVar(&mqttHost, "host", "tcp://localhost:1883", "mqtt host")
	flag.StringVar(&mqttClientId, "cid", "mqtt-exec", "mqtt client id")
	flag.StringVar(&mqttUsername, "username", "", "mqtt username")
	flag.StringVar(&mqttPassword, "password", "", "mqtt password")
	flag.UintVar(&mqttDefaultQOS, "qos", 2, "mqtt default qos")
	flag.StringVar(&configYaml, "config", "config.yaml", "exec entries config yaml")
	flag.VisitAll(func(f *flag.Flag) {
		if s := os.Getenv("MQTT_" + strings.ToUpper(f.Name)); s != "" {
			f.Value.Set(s)
		}
	})
	flag.Parse()
}

func main() {
	// read entries
	if err := unmarshalConfigYaml(); err != nil {
		log.Fatal(err)
	}
	if len(entries) == 0 {
		log.Info("there is no entry. stop.")
		return
	}

	// init mqtt client
	client := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(mqttHost).
		SetUsername(mqttUsername).
		SetPassword(mqttPassword).
		SetClientID(mqttClientId))
	defer client.Disconnect(250)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	for _, v := range entries {
		qos := byte(mqttDefaultQOS)
		if v.Qos != nil && *v.Qos <= 2 {
			qos = *v.Qos
		}

		// subscribe
		token := client.Subscribe(v.Topic, qos, v.receiveMessage)
		if token.Wait() && token.Error() != nil {
			log.Fatal(token.Error())
		}
		log.WithField("entry", v.Name).Info("an entry was loaded")
	}

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, os.Kill)
	<-sigint
}

func unmarshalConfigYaml() error {
	b, err := ioutil.ReadFile(configYaml)
	if err != nil {
		return err
	}

	var tmp map[string]*Entry
	if err := yaml.Unmarshal(b, &tmp); err != nil {
		return err
	}
	for k, v := range tmp {
		v.Name = k
		entries = append(entries, v)
	}

	return nil
}

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
