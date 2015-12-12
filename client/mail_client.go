package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/sohlich/etcd_discovery"
)

const (
	MailServiceType  = "mail"
	HttpMIMEBodyType = "application/json"
)

var (
        ErrMailServiceNotFound = fmt.Errorf("mailclient: Cannot resolve service host")
	ErrMailClientNotInitialized = fmt.Errorf("mailclient: MailClient not initialized")
)

type Email struct {
	Recipient string
	Subject   string
	Message   string
}

type MailClient interface {
	IsConnected() (bool, error)
	SendMail(recipient, subject, message string) error
}

type SuricataMailClient struct {
	discoveryClient discovery.RegistryClient
	composer        MessageComposer
}

func NewSuricataMailClient(disc discovery.RegistryClient) *SuricataMailClient {
	subjectTemp, _ := template.New("subject").Parse("Suricata: Registration confirmation")
	messageTemp, _ := template.New("message").Parse("Please confirm the registration on Suricata Talk website with click on this link {{.ConfirmationLink}}")
	return &SuricataMailClient{
		disc,
		&SuricataMessageComposer{
			subjectTemp,
			messageTemp,
		},
	}
}

func (client *SuricataMailClient) IsConnected() (bool, error) {
	if client.discoveryClient == nil {
		return false, ErrMailClientNotInitialized
	}

	services, err := client.discoveryClient.ServicesByName(MailServiceType)
	if err != nil {
		return false, err
	}

	if len(services) == 0 {
		return false, nil
	}

	return true, nil

}

func (client *SuricataMailClient) SendMail(recipient, subject, message string) error {

	// Resolve service discovery
	serviceUrl, err := client.resolveUrl()
	if err != nil {
		return err
	}

	//Compose email
	eMsg := Email{
		recipient,
		"Suricata: Registration confirmation",
		"",
	}

	// Serialize
	out, jsonError := json.Marshal(eMsg)
	if jsonError != nil {
		return err
	}
	jsonReader := strings.NewReader(string(out))

	// Send to mail microservice
	_, postErr := http.Post(serviceUrl, HttpMIMEBodyType, jsonReader)
	if postErr != nil {
		return postErr
	}

	return nil
}

func (client *SuricataMailClient) resolveUrl() (string, error) {
	mailURL, err := client.discoveryClient.ServicesByName(MailServiceType)
	if err != nil {
		return "", err
	}

	if len(mailURL) == 0 {
		return "", ErrMailServiceNotFound
	}
	return fmt.Sprintf("http://%s", mailURL[0]), nil
}

type MessageComposer interface {
	ComposeSubject(data interface{}) string
	ComposeMessage(data interface{}) string
}

type SuricataMessageComposer struct {
	subjectTemplate *template.Template
	messageTemplate *template.Template
}

func (mc *SuricataMessageComposer) ComposeSubject(data interface{}) string {
	var subject bytes.Buffer
	mc.subjectTemplate.Execute(&subject, data)
	return subject.String()
}

func (mc *SuricataMessageComposer) ComposeMessage(data interface{}) string {
	var subject bytes.Buffer
	mc.messageTemplate.Execute(&subject, data)
	return subject.String()
}
