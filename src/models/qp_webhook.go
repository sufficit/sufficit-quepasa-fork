package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
)

type QpWebhook struct {
	Url             string     `db:"url" json:"url"`                         // destination
	ForwardInternal bool       `db:"forwardinternal" json:"forwardinternal"` // forward internal msg from api
	TrackId         string     `db:"trackid" json:"trackid,omitempty"`       // identifier of remote system to avoid loop
	Failure         *time.Time `json:"failure,omitempty"`                    // first failure time
	Success         *time.Time `json:"success,omitempty"`                    // first failure time
}

var ErrInvalidResponse error = errors.New("the requested url do not return 200 status code")

func (source *QpWebhook) Post(wid string, message interface{}) (err error) {
	typeOfMessage := reflect.TypeOf(message)
	log.Infof("dispatching webhook from: (%s): %s, to: %s", typeOfMessage, wid, source.Url)

	payloadJson, _ := json.Marshal(&message)
	req, err := http.NewRequest("POST", source.Url, bytes.NewBuffer(payloadJson))
	req.Header.Set("User-Agent", "Quepasa")
	req.Header.Set("X-QUEPASA-WID", wid)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	client.Timeout = time.Second * 10
	resp, err := client.Do(req)
	if err != nil {
		log.Warn("(%s) erro ao postar no webhook: %s", wid, err.Error())
	}

	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			err = ErrInvalidResponse
		}
	}

	time := time.Now().UTC()
	if err != nil {
		if source.Failure == nil {
			source.Failure = &time
		}
	} else {
		source.Failure = nil
		source.Success = &time
	}

	return
}
