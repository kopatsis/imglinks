package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	Key string `json:"key"`
}

func getEntries() (map[string][]string, []string) {
	file, err := os.Open("SKUIMG.csv")

	if err != nil {
		log.Fatal("Error while reading the file", err)
	}

	defer file.Close()

	reader := csv.NewReader(file)

	results, err := reader.ReadAll()

	if err != nil {
		fmt.Println("Error reading csv")
	}

	ret := map[string][]string{}
	retList := []string{}

	for i, eachres := range results {
		if i != 0 {
			current := []string{}
			for j, val := range eachres {
				if j != 0 && val != "" && val != " " {
					current = append(current, val)
					retList = append(retList, val)
				}
			}
			ret[eachres[0]] = current
		}
	}

	return ret, retList
}

func writeAll(entries map[string][]string, links map[string]string) {
	csvFile, err := os.Create("imglinks.csv")
	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}
	defer csvFile.Close()
	csvwriter := csv.NewWriter(csvFile)

	for key, vals := range entries {
		currentrow := []string{key}
		currentlist := ""
		for _, name := range vals {
			if !strings.Contains(name, ".") {
				name += ".jpg"
			}

			currentlist += links[name]
			currentlist += ";"
		}
		if len(currentlist) >= 0 {
			currentlist = currentlist[:len(currentlist)-1]
		}
		currentrow = append(currentrow, currentlist)
		csvwriter.Write(currentrow)
	}
	csvwriter.Flush()

	fmt.Println("Successfully saved images links for all SKUs to CSV")
}

func ReadConfig() string {
	file, err := os.ReadFile("jkey.json")
	if err != nil {
		return ""
	}

	var config Config
	err = json.Unmarshal(file, &config)
	if err != nil {
		return ""
	}

	return config.Key
}

func WriteConfig(newkey string) {
	config := Config{Key: newkey}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Println("New Key not saved, errror: " + err.Error())
	}

	err = os.WriteFile("jkey.json", data, 0644)
	if err != nil {
		fmt.Println("New Key not saved, errror: " + err.Error())
	}
}

func testkey(key string) bool {
	if key == "" {
		return false
	}

	url := "https://api.dropboxapi.com/2/check/user"
	data := []byte(`{"query":"check"}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	fmt.Println("Checking existing access token status")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return false
	}
	defer resp.Body.Close()

	var responseMap map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseMap)
	if err != nil {
		fmt.Println("Error decoding response:", err)
		return false
	}

	if result, exists := responseMap["result"]; exists {
		return result.(string) == "check"
	} else {
		return false
	}
}

func linktype() bool {
	permlinks := true
	var entry string
	fmt.Print("Enter type of link to return (temp or permanent, default permanent): ")
	fmt.Scanln(&entry)

	if entry == "temp" {
		permlinks = false
	}

	return permlinks
}

func main() {

	linkStatusPermanent := linktype()

	existingKey := ReadConfig()

	savekey := false
	var key string
	if testkey(existingKey) {
		key = existingKey
		fmt.Println("Existing key still works")
	} else {
		fmt.Print("New Key: ")
		fmt.Scanln(&key)
		savekey = true
	}

	bearer := "Bearer " + key

	entries, entriesList := getEntries()
	name_links := map[string]string{}

	for _, val := range entries {
		for _, name := range val {
			if _, ok := name_links[name]; !ok {
				name_links[name] = ""
			}
		}
	}

	client := &http.Client{}
	contentType := "application/json"

	if linkStatusPermanent {

		for i, img := range entriesList {

			if !strings.Contains(img, ".") {
				img += ".jpg"
			}

			fmt.Print("Generating share link for: ")
			fmt.Println(img)

			url := "https://api.dropboxapi.com/2/sharing/create_shared_link_with_settings"
			path := `{"path": "/images/` + img + `"}`
			data := []byte(path)

			fakereq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
			if err != nil {
				fmt.Println(err)
				return
			}
			fakereq.Header.Add("Content-Type", contentType)
			fakereq.Header.Add("Authorization", bearer)
			fakeresp, err := client.Do(fakereq)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer fakeresp.Body.Close()

			entriesList[i] = img
		}

		for _, img := range entriesList {

			path := `{"path": "/images/` + img + `"}`
			data := []byte(path)

			url := "https://api.dropboxapi.com/2/sharing/list_shared_links"

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
			if err != nil {
				fmt.Println(err)
				return
			}
			req.Header.Add("Content-Type", contentType)
			req.Header.Add("Authorization", bearer)

			resp, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
				return
			}

			var result map[string][]map[string]string
			json.Unmarshal(body, &result)

			linklist, ok := result["links"]

			if !ok || len(linklist) == 0 {
				error_res := "Error getting link for " + img
				fmt.Println(error_res)
				name_links[img] = error_res
			} else {
				name_links[img] = result["links"][0]["url"]
				fmt.Println("Successfully received link for " + img)
			}

		}

	} else {

		for _, img := range entriesList {

			if !strings.Contains(img, ".") {
				img += ".jpg"
			}

			path := `{"path": "/images/` + img + `"}`
			data := []byte(path)

			url := "https://api.dropboxapi.com/2/files/get_temporary_link"

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
			if err != nil {
				fmt.Println(err)
				return
			}
			req.Header.Add("Content-Type", contentType)
			req.Header.Add("Authorization", bearer)

			resp, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
				return
			}

			var result map[string]any
			json.Unmarshal(body, &result)

			linklist, ok := result["link"].(string)

			if !ok || len(linklist) == 0 {
				error_res := "Error getting link for " + img
				fmt.Println(error_res)
				name_links[img] = error_res
			} else {
				name_links[img] = linklist
				fmt.Println("Successfully received link for " + img)
				fmt.Println(linklist)
			}

		}
	}

	if savekey {
		WriteConfig(key)
	}

	writeAll(entries, name_links)

}
