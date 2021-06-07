/*
Copyright 2021 The Lynx Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

type message struct {
	startTime time.Time

	expostUrl string
	mountDir  string
	message   string
	failures  []string
	pass      bool

	remoteRepo string
	refspec    string
	commitSha  string
}

func (m message) String() string {
	var msg slack.Attachment

	urlPrefix := strings.ReplaceAll(m.remoteRepo, ".git", "")
	repoName := repoFromURL(urlPrefix)
	logPath := fmt.Sprintf("%s/%s", m.expostUrl, path.Base(m.mountDir))

	msg.Pretext = fmt.Sprintf("Finish <%s|%s> e2e on elf, see more logs <%s|here>", urlPrefix, repoName, logPath)
	msg.Text = strings.Join([]string{
		fmt.Sprintf("*Commit :* <%s/commit/%s|`%s`> | %s", urlPrefix, m.commitSha, m.commitSha, m.refspec),
		fmt.Sprintf("*Result :* %s", m.message),
		fmt.Sprintf("*UseTime :* %s\n", time.Since(m.startTime)),
	}, "\n")

	if m.pass {
		msg.Color = "#2EA44F"
	} else {
		msg.Color = "#DF0000"
	}
	data, _ := json.Marshal(slack.Msg{Attachments: []slack.Attachment{msg}})
	return string(data)
}

func repoFromURL(url string) string {
	if paths := strings.Split(url, "/"); len(paths) < 2 {
		return path.Base(url)
	} else {
		return fmt.Sprintf("%s/%s", paths[len(paths)-2], paths[len(paths)-1])
	}
}

func sendMsg(hookUrl string, msg message) error {
	resp, err := http.Post(hookUrl, "Content-type: application/json", bytes.NewBufferString(msg.String()))
	if err != nil {
		return err
	}

	if err = resp.Body.Close(); err != nil {
		return fmt.Errorf("close response body: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get unexpect return code: %d", resp.StatusCode)
	}
	return nil
}

func mustSendMsg(hookUrl string, msg message, timeout time.Duration) {
	err := wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		klog.Infof("send message %s to %s", msg, hookUrl)
		if err = sendMsg(hookUrl, msg); err != nil {
			klog.Errorf("failed send messge: %s", err)
			return false, nil
		} else {
			return true, nil
		}
	})
	if err != nil {
		klog.Fatalf("unable send message: %s", err)
	}
}
