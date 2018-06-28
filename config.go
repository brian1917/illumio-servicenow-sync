package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
)

type config struct {
	IllumioFQDN          string `json:"illumioFQDN"`
	IllumioPort          int    `json:"illumioPort"`
	IllumioOrg           int    `json:"illumioOrg"`
	IllumioUser          string `json:"illumioUser"`
	IllumioKey           string `json:"illumioKey"`
	IllumioMatchField    string `json:"illumioMatchField"`
	ServiceNowURL        string `json:"serviceNowURL"`
	ServiceNowUser       string `json:"serviceNowUser"`
	ServiceNowPwd        string `json:"serviceNowPwd"`
	ServiceNowMatchField string `json:"serviceNowMatchField"`
	AppField             string `json:"appField"`
	EnvField             string `json:"envField"`
	LocField             string `json:"locField"`
	RoleField            string `json:"roleField"`
	LoggingOnly          bool   `json:"loggingOnly"`
	LogDirectory         string `json:"logDirectory"`
}

var configFile string

func init() {
	flag.StringVar(&configFile, "config", "config.json", "location of JSON configuration file")
}

func parseConfig() config {

	flag.Parse()

	//READ CONFIG FILE
	var config config

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}

	// IF A FIELD IS LEFT BLANK WE WANT TO PUT A PLACEHOLDER
	fields := []*string{&config.AppField, &config.EnvField, &config.LocField, &config.RoleField}
	for _, field := range fields {
		if *field == "" {
			*field = "csvPlaceHolderIllumio"
		}

	}

	return config
}
