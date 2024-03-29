package main

import (
	"log"
	"net/url"
	"os"
	"time"

	"github.com/brian1917/illumioapi"
)

func main() {

	// GET CONFIG
	config, pce := parseConfig()

	// SET UP LOGGING
	if len(config.Logging.LogDirectory) > 0 && config.Logging.LogDirectory[len(config.Logging.LogDirectory)-1:] != string(os.PathSeparator) {
		config.Logging.LogDirectory = config.Logging.LogDirectory + string(os.PathSeparator)
	}
	f, err := os.OpenFile(config.Logging.LogDirectory+"Illumio_ServiceNow_Sync_"+time.Now().Format("20060102_150405")+".log", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

	// LOG THE MODE
	log.Printf("INFO - Log only mode set to %t \r\n", config.Logging.LogOnly)
	if config.Logging.LogOnly == true {
		log.Printf("INFO - THIS MEANS ALL CHANGES LOGGED TO THE PCE DID NOT ACTUALLY HAPPEN. THEY WILL HAPPEN IF YOU RUN AGAIN WITH LOG ONLY SET TO FALSE.\r\n")
	}
	log.Printf("INFO - Create unmanaged workloads set to %t\r\n", config.UnmanagedWorkloads.Enable)

	// GET ALL EXISTING LABELS
	if config.Logging.verbose == true {
		log.Printf("DEBUG - Making API call to get all Labels...\r\n")
	}
	labelsAPI, apiResp, err := illumioapi.GetAllLabels(pce)

	// DEBUG LOGGING BEFORE FATAL ERROR LOGGING
	if config.Logging.verbose == true {
		log.Printf("DEBUG - Get All Labels API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL)
		log.Printf("DEBUG - Get All Labels API HTTP Reqest Header: %v \r\n", apiResp.Request.Header)
		log.Printf("DEBUG - Get All Labels API Response Status Code: %d \r\n", apiResp.StatusCode)
		log.Printf("DEBUG - Get All Labels API Response Body: \r\n %s \r\n", apiResp.RespBody)
	}
	if err != nil {
		log.Fatal(err)
	}

	accountLabelKeys := make(map[string]string)
	accountLabelValues := make(map[string]string)
	for _, l := range labelsAPI {
		accountLabelKeys[l.Href] = l.Key
		accountLabelValues[l.Href] = l.Value
	}

	// GET ALL EXISTING WORKLOADS
	if config.Logging.verbose == true {
		log.Printf("DEBUG - Making API call to get all Workloads...\r\n")
	}
	wlAPI, apiResp, err := illumioapi.GetAllWorkloads(pce)
	// DEBUG LOGGING BEFORE FATAL ERROR LOGGING
	if config.Logging.verbose == true {
		log.Printf("DEBUG - Get All Workloads API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL)
		log.Printf("DEBUG - Get All Workloads API HTTP Reqest Header: %v \r\n", apiResp.Request.Header)
		log.Printf("DEBUG - Get All Workloads API Response Status Code: %d \r\n", apiResp.StatusCode)
		log.Printf("DEBUG - Get All Workloads API Response Body:\r\n %s \r\n", apiResp.RespBody)
	}
	if err != nil {
		log.Fatal(err)
	}
	accountWorkloads := make(map[string]illumioapi.Workload)
	for _, w := range wlAPI {
		accountWorkloads[w.Href] = w
	}

	// GET DATA FROM SERVICENOW TABLE
	snURL := config.ServiceNow.TableURL + "?CSV&sysparm_fields=" + url.QueryEscape(config.ServiceNow.MatchField) + "," + url.QueryEscape(config.LabelMapping.App) +
		"," + url.QueryEscape(config.LabelMapping.Enviornment) + "," + url.QueryEscape(config.LabelMapping.Location) + "," + url.QueryEscape(config.LabelMapping.Role)

	if config.UnmanagedWorkloads.Enable == true && config.UnmanagedWorkloads.Table == "cmdb_ci_server_list" {
		snURL = snURL + ",ip_address,host_name"
	}

	data := snhttp(snURL)

	// SET THE TOTAL MATCH VARIABLE AND COUNTER
	counter := 0
	totalMatch := 0
	newUnmanagedWLs := 0

	// ITERATE THROUGH EACH LINE OF THE CSV
	for _, line := range data {
		counter++
		lineMatch := 0

		updateLabelsArray := make([]illumioapi.Label, 0)
		// CHECK IF WORKLOAD EXISTS
		for _, wl := range accountWorkloads {

			// SET SOME WORKLOAD SPECIFIC VARIABLES
			updateRequired := false
			updateLabelsArray = nil
			wlLabels := make(map[string]string)

			// SWITCH THE MATCH FIELD FROM HOSTNAME BASED ON CONFIG
			illumioMatch := wl.Hostname
			if config.Illumio.MatchField == "name" {
				illumioMatch = wl.Name
			}

			// IF THE FIRST COL (MATCH) MATHCES THE ILLUMIO MATCH, TAKE ACTION
			if line[0] == illumioMatch {
				totalMatch++
				lineMatch++
				for _, l := range wl.Labels {
					wlLabels[accountLabelKeys[l.Href]] = accountLabelValues[l.Href]
				}

				// CHECK EACH LABEL TYPE TO SEE IF IT NEEDS TO BE UPDATED
				labelKeys := []string{"app", "env", "loc", "role"}
				configFields := []string{config.LabelMapping.App, config.LabelMapping.Enviornment, config.LabelMapping.Location, config.LabelMapping.Role}

				// ITERATE THROUGH EACH LABEL TYPE
				for i := 0; i <= 3; i++ {

					// CANNOT BE "csvPlaceHolderIllumio" (SKIPPING THAT COL) AND THE LABELS DON'T MATCH
					if configFields[i] != "csvPlaceHolderIllumio" && wlLabels[labelKeys[i]] != line[i+1] {
						log.Printf("INFO - %s - %s label updated from %s to %s\r\n", wl.Hostname, labelKeys[i], wlLabels[labelKeys[i]], line[i+1])
						updateRequired = true

						// IF THE NEW VALUE (FROM SN) IS BLANK, WE DON'T APPEND TO THE UPDATE ARRAY
						if line[i+1] != "" {
							updateLabelsArray = append(updateLabelsArray, illumioapi.Label{Key: labelKeys[i], Value: line[i+1]})
						}

						// ADD EXISTING LABEL IF IT EXISTS
					} else if line[i+1] != "" {
						updateLabelsArray = append(updateLabelsArray, illumioapi.Label{Key: labelKeys[i], Value: wlLabels[labelKeys[i]]})
					}
				}

				// UPDATE THE WORKLOAD IF ANYTHING NEEDS TO CHANGE
				if updateRequired == true {
					if config.Logging.verbose == true {
						log.Printf("DEBUG - Updating workload %s ...\r\n", wl.Hostname)
					}
					updateWorkload(updateLabelsArray, wl)
				}
			}

		}

		// IF THERE WERE NO MATCHES AND IT'S NOT THE HEADER FILE, CREATE THE UNMANAGED WORKLOAD
		if lineMatch == 0 && counter != 1 && config.UnmanagedWorkloads.Enable == true {
			interfaceList := []string{"eth0"}
			ipAddressList := []string{line[5]}
			if len(ipAddressList[0]) == 0 || len(line[0]) == 0 {
				log.Printf("WARNING - Not enough information to create unmanaged workload for hostname %s\r\n", line[0])
			} else {
				err := createUnmanagedWorkload(interfaceList, ipAddressList, line[1], line[2], line[3], line[4], line[0])
				if err == nil {
					newUnmanagedWLs++
				}

			}
		}

	}
	// SUMMARIZE ACTIONS FOR LOG
	log.Printf("INFO - %d total servers in CMDB and %d matched to PCE workloads\r\n", len(data)-1, totalMatch)
	if config.UnmanagedWorkloads.Enable == true {
		log.Printf("INFO - %d new unmanaged workloads created\r\n", newUnmanagedWLs)
		log.Printf("INFO - %d servers with not enough info for unmanaged workload.\r\n", len(data)-1-totalMatch-newUnmanagedWLs)
	}
}
