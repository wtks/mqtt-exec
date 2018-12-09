package main

import (
	"github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/signal"
)

const (
	MQTTHostKey       = "MQTT_HOST"
	MQTTClientIDKey   = "MQTT_CLIENT_ID"
	MQTTUsernameKey   = "MQTT_USERNAME"
	MQTTPasswordKey   = "MQTT_PASSWORD"
	MQTTDefaultQOS    = "MQTT_QOS"
	ExecConfigYamlKey = "EXEC_CONFIG_YAML"
)

var (
	entries []*Entry
)

func init() {
	viper.SetDefault(MQTTHostKey, "tcp://localhost:1883")
	viper.SetDefault(MQTTClientIDKey, "mqtt-exec")
	viper.SetDefault(MQTTUsernameKey, "")
	viper.SetDefault(MQTTPasswordKey, "")
	viper.SetDefault(MQTTDefaultQOS, 2)
	viper.SetDefault(ExecConfigYamlKey, "./config.yaml")
	pflag.String("h", "", "mqtt host")
	pflag.String("c", "", "mqtt client id")
	pflag.String("u", "", "mqtt username")
	pflag.String("p", "", "mqtt password")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	viper.AutomaticEnv()
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
		AddBroker(viper.GetString(MQTTHostKey)).
		SetUsername(viper.GetString(MQTTUsernameKey)).
		SetPassword(viper.GetString(MQTTPasswordKey)).
		SetClientID(viper.GetString(MQTTClientIDKey)))
	defer client.Disconnect(250)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	for _, v := range entries {
		qos := byte(viper.GetInt(MQTTDefaultQOS))
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
	b, err := ioutil.ReadFile(viper.GetString(ExecConfigYamlKey))
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
