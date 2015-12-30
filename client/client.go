package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/nats-io/nats"
	"github.com/sohlich/etcd_discovery"
)

const (
	MailServiceType = "mail"
)

var (
	ErrMailServiceNotFound      = fmt.Errorf("mailclient: Cannot resolve service host")
	ErrMailClientNotInitialized = fmt.Errorf("mailclient: MailClient not initialized")
)

type Email struct {
	Recipient string
	Subject   string
	Message   string
}

type MailClient interface {
	IsConnected() (bool, error)
	SendMail(recipient string, subject, message interface{}) error
}

type MessageComposer interface {
	ComposeSubject(data interface{}) string
	ComposeMessage(data interface{}) string
}

type SuricataMessageComposer struct {
	SubjectTemplate *template.Template
	MessageTemplate *template.Template
}

func (mc *SuricataMessageComposer) ComposeSubject(data interface{}) string {
	var subject bytes.Buffer
	mc.SubjectTemplate.Execute(&subject, data)
	return subject.String()
}

func (mc *SuricataMessageComposer) ComposeMessage(data interface{}) string {
	var subject bytes.Buffer
	mc.MessageTemplate.Execute(&subject, data)
	return subject.String()
}

// REST Client
const (
	HttpMIMEBodyType = "application/json"
)

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

func (client *SuricataMailClient) SendMail(recipient string, subject, message interface{}) error {

	// Resolve service discovery
	serviceURL, err := client.resolveUrl()
	if err != nil {
		return err
	}

	//Compose email
	eMsg := Email{
		Recipient: recipient,
		Subject:   client.composer.ComposeSubject(subject),
		Message:   client.composer.ComposeMessage(message),
	}

	// Serialize
	out, jsonError := json.Marshal(eMsg)
	if jsonError != nil {
		return err
	}
	jsonReader := strings.NewReader(string(out))

	// Send to mail microservice
	_, postErr := http.Post(serviceURL, HttpMIMEBodyType, jsonReader)
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

// NATS Client
type NatsMailClient struct {
	conn        *nats.Conn
	encodedConn *nats.EncodedConn
	composer    MessageComposer
}

func NewNatsMailClient() *NatsMailClient {
	nc, _ := nats.Connect(nats.DefaultURL)
	conn, _ := nats.NewEncodedConn(nc, nats.GOB_ENCODER)
	// defer conn.Close() TODO on close client
	subjectTemp, _ := template.New("subject").Parse("Suricata: Registration confirmation")
	messageTemp, _ := template.New("message").Parse("Please confirm the registration on Suricata Talk website with click on this link {{.ConfirmationLink}}")
	return &NatsMailClient{
		nc,
		conn,
		&SuricataMessageComposer{
			subjectTemp,
			messageTemp,
		},
	}
}

func (client *NatsMailClient) IsConnected() (bool, error) {
	status := client.conn.Status() == nats.CONNECTED
	return status, nil
}

func (client *NatsMailClient) SendMail(recipient string, subject, message interface{}) error {
	//Compose email
	eMsg := &Email{
		Recipient: recipient,
		Subject:   client.composer.ComposeSubject(subject),
		Message:   client.composer.ComposeMessage(message),
	}
	err := client.encodedConn.Publish(MailServiceType, eMsg)
	return err
}
