package client

import (
	"fmt"
	stdLog "log"
	"net/http"
	"testing"
	"text/template"
	"time"
)

func TestMailComposer(t *testing.T) {

	subjectTemp, _ := template.New("subject").Parse("Suricata: Registration confirmation")
	messageTemp, _ := template.New("message").Parse("Please confirm the registration on Suricata Talk website with click on this link {{.ConfirmationLink}}")

	composer := SuricataMessageComposer{
		subjectTemp,
		messageTemp,
	}

	msg := composer.ComposeMessage(struct{ ConfirmationLink string }{"http://127.0.0.1:8080/confirm"})
	subj := composer.ComposeSubject(struct{}{})

	if msg != "Please confirm the registration on Suricata Talk website with click on this link http://127.0.0.1:8080/confirm" {
		fmt.Println(msg)
		t.Error("Message bad composed")
	}

	if subj != "Suricata: Registration confirmation" {
		t.Error("Subject bad composed")
	}

}

// FakeRegistryClient to resolve fake
// mail service
type FakeRegistryClient struct {
}

func (fc *FakeRegistryClient) Register() error {
	return nil
}

func (fc *FakeRegistryClient) ServicesByName(name string) ([]string, error) {
	stdLog.Println("Returning service.")
	return []string{"127.0.0.1:3030"}, nil
}

func (fc *FakeRegistryClient) Unregister() error {
	return nil
}

func TestMailClient(t *testing.T) {
	rC := FakeRegistryClient{}
	mailClient := NewSuricataMailClient(&rC)
	condition := make(chan struct{}, 0)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		stdLog.Println("Getting request")
		go func() { condition <- struct{}{} }()
	})
	stdLog.Println("Waiting for response")
	go http.ListenAndServe(":3030", mux)
	mailClient.SendMail("sohlich@gmail.com", "Subj", "Message")
	ticker := time.NewTicker(time.Duration(10) * time.Second)

	//
	select {
	case <-condition:
		return
	case <-ticker.C:
		ticker.Stop()
		t.Error("Did not get any request in time.")
		return
	}
}
