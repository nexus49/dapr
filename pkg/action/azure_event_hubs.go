package action

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"

	eventhub "github.com/Azure/azure-event-hubs-go"
	log "github.com/Sirupsen/logrus"
)

type AzureEventHubs struct {
	Spec EventSourceSpec
}

type AzureEventHubsMetadata struct {
	ConnectionString string `json:"connectionString"`
}

func NewAzureEventHubs() *AzureEventHubs {
	return &AzureEventHubs{}
}

func (a *AzureEventHubs) Init(eventSourceSpec EventSourceSpec) error {
	a.Spec = eventSourceSpec
	return nil
}

func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *AzureEventHubs) Write(data interface{}) error {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	b, err := json.Marshal(a.Spec.ConnectionInfo)
	if err != nil {
		return err
	}

	var connInfo AzureEventHubsMetadata
	err = json.Unmarshal(b, &connInfo)
	if err != nil {
		return err
	}

	connStr := connInfo.ConnectionString

	hub, err := eventhub.NewHubFromConnectionString(connStr)
	if err != nil {
		return err
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = hub.Send(context.Background(), &eventhub.Event{
		Data: dataBytes,
	})
	if err != nil {
		return err
	}

	log.Info("EventHubs event sent successfully")
	return nil
}

func (a *AzureEventHubs) Read(metadata interface{}) (interface{}, error) {
	return nil, nil
}

func (a *AzureEventHubs) ReadAsync(metadata interface{}, callback func([]byte) error) error {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	b, err := json.Marshal(a.Spec.ConnectionInfo)
	if err != nil {
		return err
	}

	var connInfo AzureEventHubsMetadata
	err = json.Unmarshal(b, &connInfo)
	if err != nil {
		return err
	}

	connStr := connInfo.ConnectionString

	hub, err := eventhub.NewHubFromConnectionString(connStr)
	if err != nil {
		return err
	}

	handler := func(c context.Context, event *eventhub.Event) error {
		return callback(event.Data)
	}

	ctx := context.Background()
	runtimeInfo, err := hub.GetRuntimeInformation(ctx)
	if err != nil {
		return err
	}

	for _, partitionID := range runtimeInfo.PartitionIDs {
		_, err := hub.Receive(ctx, partitionID, handler, eventhub.ReceiveWithLatestOffset())
		if err != nil {
			return err
		}
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, os.Kill)
	<-signalChan

	hub.Close(context.Background())

	return nil
}