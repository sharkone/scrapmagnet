package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"

	"github.com/dukex/mixpanel"
)

var (
	publicIP = ""
)

func peopleSet() {
	properties := make(map[string]interface{})
	properties["Server OS"] = runtime.GOOS
	properties["Server Arch"] = runtime.GOARCH

	if settings.mixpanelData != "" {
		if data, err := base64.StdEncoding.DecodeString(settings.mixpanelData); err == nil {
			json.Unmarshal([]byte(data), &properties)
		} else {
			log.Print(err)
		}
	}

	client := mixpanel.NewMixpanel(settings.mixpanelToken)
	people := client.Identify(getDistinctId())
	people.Update("$set", properties)
}

func trackingEvent(eventName string, properties map[string]interface{}, mixpanelData string) {
	properties["Server OS"] = runtime.GOOS
	properties["Server Arch"] = runtime.GOARCH

	if settings.mixpanelData != "" {
		if data, err := base64.StdEncoding.DecodeString(settings.mixpanelData); err == nil {
			json.Unmarshal([]byte(data), &properties)
		} else {
			log.Print(err)
		}
	}

	if mixpanelData != "" {
		if data, err := base64.StdEncoding.DecodeString(mixpanelData); err == nil {
			json.Unmarshal([]byte(data), &properties)
		} else {
			log.Print(err)
		}
	}

	client := mixpanel.NewMixpanel(settings.mixpanelToken)
	client.Track(getDistinctId(), eventName, properties)
}

func getPublicIP() string {
	if publicIP == "" {
		if resp, err := http.Get("http://myexternalip.com/raw"); err == nil {
			defer resp.Body.Close()
			if content, err := ioutil.ReadAll(resp.Body); err == nil {
				publicIP = string(content)
			} else {
				log.Panic(err)
			}
		} else {
			log.Panic(err)
		}
	}

	return publicIP
}

func getDistinctId() string {
	data := []byte(runtime.GOOS + runtime.GOARCH + getPublicIP())
	return fmt.Sprintf("%x", sha1.Sum(data))
}
