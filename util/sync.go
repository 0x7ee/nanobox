// Copyright (c) 2015 Pagoda Box Inc
//
// This Source Code Form is subject to the terms of the Mozilla Public License, v.
// 2.0. If a copy of the MPL was not distributed with this file, You can obtain one
// at http://mozilla.org/MPL/2.0/.
//

package util

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nanobox-io/golang-mist"
	"github.com/nanobox-io/nanobox-cli/config"
	"github.com/nanobox-io/nanobox-golang-stylish"
)

type (

	// nsync
	Sync struct {
		ID      string `json:"id"`
		Model   string
		Path    string
		Status  string `json:"status"`
		Verbose bool
	}

	// entry
	entry struct {

		// 'model' message fields
		Action   string `json:"action"`
		Document string `json:"document"`
		Model    string `json:"model"`

		// 'log' message fields
		Content  string `json:"content"`
		Priority int    `json:"priority"`
		Time     string `json:"time"`
		Type     string `json:"type"`
	}
)

// run issues a sync to the running nanobox VM
func (s *Sync) Run(opts []string) {

	// connect 'mist' to the server running on the guest machine
	client, err := mist.NewRemoteClient(config.MistURI)
	if err != nil {
		config.Fatal("[utils/sync] client.Connect() failed ", err.Error())
	}
	defer client.Close()

	// subscribe to job updates
	jobTags := []string{"job", s.Model}
	if err := client.Subscribe(jobTags); err != nil {
		fmt.Printf(stylish.ErrBullet("Nanobox failed to subscribe to app logs. Your sync will continue as normal, and log output is available on your dashboard."))
	}
	defer client.Unsubscribe(jobTags)

	logLevel := "info"
	if s.Verbose {
		logLevel = "debug"
	}

	// subscirbe to deploy logs; if verbose, also subscribe to the 'debug' logs
	logTags := []string{"log", "deploy", logLevel}
	if err := client.Subscribe(logTags); err != nil {
		fmt.Printf(stylish.ErrBullet("Nanobox failed to subscribe to debug logs. Your sync will continue as normal, and log output is available on your dashboard."))
	}
	defer client.Unsubscribe(logTags)

	//
	// issue a sync
	res, err := http.Post(s.Path, "text/plain", nil)
	if err != nil {
		config.Fatal("[utils/sync] api.DoRawRequest() failed", err.Error())
	}
	defer res.Body.Close()

	// handle
stream:
	for msg := range client.Messages() {

		// unmarshal the incoming message; a message will be of two possible 'types'
		// 1. A log entry that has type, time, priority, and content
		// 2. A 'model' update with model, action, and document
		// once parsed, the following case statemt will determine what time of message
		// was received and what to do about it.
		e := &entry{}
		if err := json.Unmarshal([]byte(msg.Data), &e); err != nil {
			config.Fatal("[utils/sync] json.Unmarshal() failed", err.Error())
		}

		// depending on what fields the data has, determines what needs to happen...
		switch {

		// if the message contains the log field, the log is printed. The message is
		// then checked to see if it contains a model field...
		// example entry: {Time: "time", Log: "content"}
		case e.Content != "":
			fmt.Printf(e.Content)

			// if the message contains the model field...
		case e.Model != "":

			// update the model status
			if err := json.Unmarshal([]byte(e.Document), s); err != nil {
				config.Fatal("[utils/sync] json.Unmarshal() failed ", err.Error())
			}

			// break the stream once we get a model update. If we ever have intermediary
			// statuses we can throw in a case that will handle this on a status-by-status
			// basis (current statuses: complete, unavailable, errored)
			break stream

		// nanobox didn't know what to do with this message
		default:
			fmt.Printf(stylish.ErrBullet("Malformed Entry..."))
			// break stream
		}
	}
}
