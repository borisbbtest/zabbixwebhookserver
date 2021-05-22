package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	zabbix "mpk.lcl/zabbix-webhook/modules/zbx"
)

var log = logrus.WithField("context", "webhook")

type WebHook struct {
	channel chan *HookRequest
	config  WebHookConfig
}

type WebHookConfig struct {
	Port                 int    `yaml:"port"`
	QueueCapacity        int    `yaml:"queueCapacity"`
	ZabbixServerHost     string `yaml:"zabbixServerHost"`
	ZabbixServerPort     int    `yaml:"zabbixServerPort"`
	ZabbixHostDefault    string `yaml:"zabbixHostDefault"`
	ZabbixHostAnnotation string `yaml:"zabbixHostAnnotation"`
	ZabbixKeyPrefix      string `yaml:"zabbixKeyPrefix"`
}

type HookRequest struct {
	TAlert       string `json:"alert_type"`
	NameAlert    string `json:"alert_name"`
	SearchPeriod string `json:"search_period"`
	HitOpertor   string `json:"hit_oeprator"`
	Status       string `json:"sev"`
	Messages     string `json:"messages"`
}

func New(cfg *WebHookConfig) *WebHook {

	return &WebHook{
		channel: make(chan *HookRequest, cfg.QueueCapacity),
		config:  *cfg,
	}
}

func ConfigFromFile(filename string) (cfg *WebHookConfig, err error) {
	log.Infof("Loading configuration at '%s'", filename)
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("can't open the config file: %s", err)
	}

	// Default values
	config := WebHookConfig{
		Port:                 8080,
		QueueCapacity:        500,
		ZabbixServerHost:     "127.0.0.1",
		ZabbixServerPort:     10051,
		ZabbixHostAnnotation: "zabbix_host",
		ZabbixKeyPrefix:      "prometheus",
		ZabbixHostDefault:    "",
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, fmt.Errorf("can't read the config file: %s", err)
	}

	log.Info("Configuration loaded")
	return &config, nil
}

func (hook *WebHook) Start() error {

	// Launch the process thread
	go hook.processAlerts()

	// Launch the listening thread
	log.Println("Initializing HTTP server")
	http.HandleFunc("/alerts", hook.alertsHandler)
	err := http.ListenAndServe(":"+strconv.Itoa(hook.config.Port), nil)
	if err != nil {
		return fmt.Errorf("can't start the listening thread: %s", err)
	}

	log.Info("Exiting")
	close(hook.channel)

	return nil
}

func (hook *WebHook) alertsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		hook.postHandler(w, r)
	default:
		http.Error(w, "unsupported HTTP method only post send", 400)
	}
}

func (hook *WebHook) postHandler(w http.ResponseWriter, r *http.Request) {

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatalln(err)
	}
	// fmt.Println(string(bytes))

	defer r.Body.Close()
	var m HookRequest
	if err := json.Unmarshal(bytes, &m); err != nil {
		log.Errorf("error decoding message: %v", err)
		http.Error(w, "request body is not valid json", 400)
		return
	}

	hook.channel <- &m
	fmt.Printf(m.Status)

}
func (hook *WebHook) processAlerts() {
	log.Info("Alerts queue started")
	// While there are alerts in the queue, batch them and send them over to Zabbix
	var metrics []*zabbix.Metric
	for {
		select {
		case a := <-hook.channel:
			if a == nil {
				log.Info("Queue Closed")
				return
			}
			host := hook.config.ZabbixHostAnnotation
			// Send alerts only if a host annotation is present or configuration for default host is not empty
			if host != "" {
				key := fmt.Sprintf("%s.%s", hook.config.ZabbixKeyPrefix, a.Status)
				value := a.NameAlert
				log.Infof("added Zabbix metrics, host: '%s' key: '%s', value: '%s'", host, key, value)
				metrics = append(metrics, zabbix.NewMetric(host, key, value))
			}
		default:
			if len(metrics) != 0 {
				hook.zabbixSend(metrics)
				metrics = metrics[:0]
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (hook *WebHook) zabbixSend(metrics []*zabbix.Metric) {
	// Create instance of Packet class
	packet := zabbix.NewPacket(metrics)

	// Send packet to zabbix
	log.Infof("sending to zabbix '%s:%d'", hook.config.ZabbixServerHost, hook.config.ZabbixServerPort)
	z := zabbix.NewSender(hook.config.ZabbixServerHost, hook.config.ZabbixServerPort)
	_, err := z.Send(packet)
	if err != nil {
		log.Error(err)
	} else {
		log.Info("successfully sent")
	}

}
