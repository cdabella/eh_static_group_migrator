package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/tonyHuinker/ehop"
)

func askForInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(prompt)
	response, _ := reader.ReadString('\n')
	fmt.Println("")
	return strings.TrimSpace(response)
}

type deviceGroup struct {
	Description          string      `json:"description"`
	ID                   json.Number `json:"id,Number"`
	Name                 string      `json:"name"`
	IncludeCustomDevices bool        `json:"include_custom_devices"`
	Dynamic              bool        `json:"dynamic"`
	Field                string      `json:"field"`
	Value                string      `json:"value"`
}

type device struct {
	Ipaddr4 string `json:"ipaddr4"`
	Ipaddr6 string `json:"ipaddr6"`
}

type keyDetail struct {
	KeyType   string `json:"key_type"`
	Addr      string `json:"addr"`
	DeviceOID int    `json:"device_oid"`
}

func getDeviceGroupIPs(ID string, eh *ehop.EDA) []string {
	resp, error := ehop.CreateEhopRequest("GET", "devicegroups/"+ID+"/devices", ``, eh)
	defer resp.Body.Close()

	if error != nil {
		fmt.Println("Error requesting device IPs from device group " + ID + ": " + error.Error())
		os.Exit(-1)
	} else if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-200 status code requesting device IPs from device group " + ID + ": " + resp.Status)
		os.Exit(-1)
	}

	var deviceList []device

	error = json.NewDecoder(resp.Body).Decode(&deviceList)
	if error != nil {
		fmt.Println("Error parsing DeviceList JSON: " + error.Error())
		os.Exit(-1)
	}

	var ipList []string

	for _, device := range deviceList {
		ipList = append(ipList, device.Ipaddr4)
	}

	return ipList
}

func findDeviceID(ip string, eh *ehop.EDA) string {
	resp, error := ehop.CreateEhopRequest("GET", "devices?search_type=ip%20address&value="+ip, ``, eh)
	defer resp.Body.Close()

	if error != nil {
		fmt.Println("Error searching for device ID: " + error.Error())
		os.Exit(-1)
	} else if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-200 status code searching for device: " + resp.Status)
		os.Exit(-1)
	}

	var devices []ehop.Device

	error = json.NewDecoder(resp.Body).Decode(&devices)
	if error != nil {
		fmt.Println("Error parsing Device JSON: " + error.Error())
		os.Exit(-1)
	}

	if len(devices) > 1 {
		fmt.Println("More than one device with IP " + ip + ". Using first device.")

	} else if len(devices) == 0 {
		return ""
	}

	return strconv.Itoa(devices[0].ID)
}

// Looks up a device group by name. If the device group exists, returns the
// device group ID, otherwise creates the device group and returns the new
// device group ID.
func addDeviceGroup(dg deviceGroup, eh *ehop.EDA) string {
	resp, error := ehop.CreateEhopRequest("GET", "devicegroups?all=false&name="+dg.Name, ``, eh)

	defer resp.Body.Close()

	if error != nil {
		fmt.Println("Error requesting device group " + dg.Name + " from new system: " + error.Error())
		os.Exit(-1)
	} else if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-200 status code requesting device group " + dg.Name + " from new system: " + resp.Status)
		os.Exit(-1)
	}

	var deviceGroupList []deviceGroup

	error = json.NewDecoder(resp.Body).Decode(&deviceGroupList)
	if error != nil {
		fmt.Println("Error parsing DeviceGroupList JSON: " + error.Error())
		os.Exit(-1)
	}

	if len(deviceGroupList) == 0 {
		body := `{"description": "` + dg.Description + `","dynamic": false, "include_custom_devices":` + strconv.FormatBool(dg.IncludeCustomDevices) + `,"name": "` + dg.Name + `}`
		resp, error = ehop.CreateEhopRequest("POST", "devicegroups", body, eh)
		defer resp.Body.Close()

		if error != nil {
			fmt.Println("Error creating device group " + dg.Name + " on new system: " + error.Error())
			os.Exit(-1)
		} else if resp.StatusCode != http.StatusCreated {
			fmt.Println("Non-201 status code creating device group " + dg.Name + " on new system: " + resp.Status)
			os.Exit(-1)
		}
		loc := resp.Header.Get("location")
		splitLoc := strings.Split(loc, "/")
		return splitLoc[len(splitLoc)-1]
	}

	for _, potentialDG := range deviceGroupList {
		if dg.Name == potentialDG.Name {
			return string(potentialDG.ID)
		}
	}

	return ""

}

// Adds the device with device ID deviceID to the device group. Returns true
// if successfull otherwise false
func addDeviceToDevice(groupID string, deviceID string, eh *ehop.EDA) bool {
	return true
}

func main() {
	//Specify Key File
	keyFile := askForInput("What is the source EDA/ECA keyFile?")
	srcEDA := ehop.NewEDAfromKey(keyFile)

	keyFile = askForInput("What is the destion EDA/ECA keyFile?")
	dstEDA := ehop.NewEDAfromKey(keyFile)

	filter := askForInput("Device group name filter? Leave blank for no filter")

	//Get all devices from the system, filtered with user input
	resp, error := ehop.CreateEhopRequest("GET", "devicegroups?all=false&name="+filter, ``, srcEDA)
	defer resp.Body.Close()

	if error != nil {
		fmt.Println("Error requesting device groups: " + error.Error())
		os.Exit(-1)
	} else if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-200 status code requesting peer metrics: " + resp.Status)
		os.Exit(-1)
	}

	//Store device groups into Structs
	var deviceGroupList []deviceGroup

	error = json.NewDecoder(resp.Body).Decode(&deviceGroupList)
	if error != nil {
		fmt.Println("Error parsing DeviceGroupList JSON: " + error.Error())
		os.Exit(-1)
	}
	fmt.Println("Devices Groups successfully queried.")
	fmt.Println("Total Groups (including Dynamic): " + strconv.Itoa(len(deviceGroupList)))
	fmt.Println("Filtering Static Groups and adding appropriate devices")
	dgCounter := 0
	for _, dg := range deviceGroupList {
		if !dg.Dynamic {
			dgCounter++

			newDeviceGroupID := addDeviceGroup(dg, dstEDA)

			fmt.Println(dg.Name + " -- " + dg.Description)
			deviceIPList := getDeviceGroupIPs(string(dg.ID), srcEDA)

			deviceCounter := 0
			for _, IP := range deviceIPList {
				dstDeviceID := findDeviceID(IP, dstEDA)
				if dstDeviceID == "" {
					fmt.Println("Device " + IP + " not found for device group " + newDeviceGroupID)
				} else {
					deviceCounter++
					body := `{"assign": [` + dstDeviceID + `]}`
					resp, error = ehop.CreateEhopRequest("POST", "devicegroups/"+newDeviceGroupID+"/devices", body, srcEDA)
					defer resp.Body.Close()

					if error != nil {
						fmt.Println("Error assigning device " + IP + " to device group " + newDeviceGroupID + ": " + error.Error())
						os.Exit(-1)
					} else if resp.StatusCode != http.StatusNoContent {
						fmt.Println("Non-204 status code requesting peer metrics: " + resp.Status)
						//os.Exit(-1)
					}
				}
			}
			fmt.Println(strconv.Itoa(deviceCounter) + " devices migrated")
			fmt.Println("")
		}

	}
	fmt.Println(strconv.Itoa(dgCounter) + " device groups migrated")
	fmt.Println("")
}
