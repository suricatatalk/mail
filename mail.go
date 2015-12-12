package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"
	"github.com/mailgun/mailgun-go"
	"github.com/sebest/logrusly"
	"github.com/sohlich/etcd_discovery"
	"golang.org/x/net/context"
)

const (
	ServiceName = "mail"
	TokenHeader = "X-AUTH"

	//Configuration keys
	KeyLogly = "LOGLY_TOKEN"
)

var (
	// ErrMailerNotInitialized is
	ErrMailerNotInitialized = fmt.Errorf("mailgunmailer: Mailer not initialized yet")

	// Configs
	etcdConfig = &EtcdConfig{}
	appConfig  = &AppConfig{}

	// Service discovery vars
	registryConfig discovery.EtcdRegistryConfig = discovery.EtcdRegistryConfig{
		ServiceName: ServiceName,
	}
	registryClient *discovery.EtcdReigistryClient
)

type AppConfig struct {
	Host   string `default:"127.0.0.1"`
	Port   string `default:"5050"`
	Name   string `default:"mail1"`
	Domain string
	ApiKey string
	Sender string
}

type EtcdConfig struct {
	Endpoint string `default:"http://127.0.0.1:4001"`
}

type Mailer interface {
	SendMail(subject, message, recipient string) error
	Close()
}

func loadConfig(config *AppConfig, etcd *EtcdConfig) {

	err := envconfig.Process("mail", config)
	if err != nil {
		log.Panic(err)
	}

	err = envconfig.Process("etcd", etcd)
	if err != nil {
		log.Panic(err)
	}

	if len(os.Getenv(KeyLogly)) > 0 {
		hook := logrusly.NewLogglyHook(os.Getenv(KeyLogly),
			config.Host,
			log.InfoLevel,
			config.Name)
		log.AddHook(hook)
	}

}

func main() {

	loadConfig(appConfig, etcdConfig)

	log.SetLevel(log.DebugLevel)

	var registryErr error
	log.Infoln("Initializing service discovery client for %s", appConfig.Name)
	registryConfig.InstanceName = appConfig.Name
	registryConfig.BaseURL = fmt.Sprintf("%s:%s", appConfig.Host, appConfig.Port)
	registryConfig.EtcdEndpoints = []string{etcdConfig.Endpoint}
	registryClient, registryErr = discovery.New(registryConfig)
	if registryErr != nil {
		log.Panic(registryErr)
	}
	registryClient.Register()

	log.SetLevel(log.DebugLevel)
	mailer := NewMailGun(appConfig.Domain, appConfig.ApiKey, appConfig.Sender)
	http.HandleFunc("/", MailerFunc(mailer))
	http.ListenAndServe(":5050", nil)
}

func MailerFunc(m Mailer) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		mail := mailStruct{}
		decoder := json.NewDecoder(req.Body)
		decoder.Decode(&mail)
		log.Infoln("Sending mail %v", mail)
		m.SendMail(mail.Subject, mail.Message, mail.Recipient)
	}
}

type mailStruct struct {
	Sender    string
	Message   string
	Subject   string
	Recipient string
}

type MailGunMailer struct {
	mailgun.Mailgun
	sendChannel chan mailStruct
	sender      string
	cancel      context.CancelFunc
}

func NewMailGun(domain, apiKey, sender string) Mailer {
	mg := mailgun.NewMailgun(domain, apiKey, "")
	senderChan := make(chan mailStruct, 0)
	ctx, cancel := context.WithCancel(context.TODO())
	mailer := MailGunMailer{
		mg,
		senderChan,
		sender,
		cancel,
	}
	go func() {
		for {
			log.Debug("Waiting for message")
			select {
			case m := <-senderChan:
				log.Debug("Receiving message %v", m)
				message := mailgun.NewMessage(m.Sender, m.Subject, m.Message, m.Recipient)
				response, id, err := mg.Send(message)
				if err != nil {
					log.Errorln(err)
				}
				log.Infof("Sending email to recipient %s\nreponse %s\nid %s", m.Recipient, response, id)
			case <-ctx.Done():
				log.Infoln("Closing goroutine to send mails")
				return
			}

		}
	}()
	return &mailer
}

func (mgm *MailGunMailer) SendMail(subject, message, recipient string) error {
	if mgm.sendChannel == nil {
		return ErrMailerNotInitialized
	}

	mgm.sendChannel <- mailStruct{
		mgm.sender,
		subject,
		message,
		recipient,
	}

	return nil
}

func (mgm *MailGunMailer) Close() {
	mgm.cancel()
}
