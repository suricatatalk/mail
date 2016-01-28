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
	SendMail(recipient, subject, message string) error
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

func NewMailComposer(sbjTmp, msgTmp *template.Template) *SuricataMessageComposer {
	return &SuricataMessageComposer{
		sbjTmp,
		msgTmp,
	}
}

// REST Client
const (
	HttpMIMEBodyType = "application/json"
)

type SuricataMailClient struct {
	discoveryClient discovery.RegistryClient
}

func NewSuricataMailClient(disc discovery.RegistryClient) *SuricataMailClient {
	// subjectTemp, _ := template.New("subject").Parse("Suricata: Registration confirmation")
	// messageTemp, _ := template.New("message").Parse("Please confirm the registration on Suricata Talk website with click on this link {{.ConfirmationLink}}")
	return &SuricataMailClient{
		disc,
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
	serviceURL, err := client.resolveUrl()
	if err != nil {
		return err
	}

	//Compose email
	eMsg := Email{
		Recipient: recipient,
		Subject:   subject,
		Message:   message,
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
}

func NewNatsMailClient(url string) (*NatsMailClient, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	conn, encodeErr := nats.NewEncodedConn(nc, nats.GOB_ENCODER)
	if encodeErr != nil {
		return nil, encodeErr
	}

	// defer conn.Close() TODO on close client

	return &NatsMailClient{
		nc,
		conn,
	}, nil
}

func (client *NatsMailClient) IsConnected() (bool, error) {
	status := client.conn.Status() == nats.CONNECTED
	return status, nil
}

func (client *NatsMailClient) SendMail(recipient, subject, message string) error {
	//Compose email
	eMsg := &Email{
		Recipient: recipient,
		Subject:   subject,
		Message:   message,
	}
	err := client.encodedConn.Publish(MailServiceType, eMsg)
	return err
}
