package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	host    = "<api server>"
	apikey  = "<api key>"
	version = "0.4"
)

func versionCheck() {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	response, err := client.Get(fmt.Sprintf("%s/latest-version", host))
	if err != nil {
		fmt.Println("Error fecthing latest version, using old")
		return
	}
	defer response.Body.Close()
	newVersion, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error fecthing latest version, using old")
		return
	}
	if strings.TrimSpace(string(newVersion)) == version {
		fmt.Println("Using latest version")

		return

	}
	fmt.Println("New version detected, downloading it now...")

	response2, err := client.Get(fmt.Sprintf("%s/latest", host))
	if err != nil {
		log.Println("Error downloading new version, aborting")
		return
	}
	defer response2.Body.Close()
	out, err := os.Create("run_flexx.latest.exe")
	if err != nil {
		log.Println("Error downloading new version, aborting")
		return
	}
	defer out.Close()
	_, err = io.Copy(out, response2.Body)
	if err != nil {
		log.Println("Error downloading new version, aborting")
		return
	}
	fmt.Println("Update downloaded")
	cmd := exec.Command("cmd", "/C", "start", "update.bat")
	err = cmd.Start()
	if err != nil {
		log.Println("Error replacing with new version, aborting")
		return
	}
	os.Exit(0)
	/*procAttr := os.ProcAttr{}
	devNull, _ := os.Open(os.DevNull)
	procAttr.Files = []*os.File{os.Stdin, devNull, os.Stderr}
	p, err := os.StartProcess("update.bat", []string{}, &procAttr)
	if err != nil {
		log.Println("Error replacing with new version, aborting")
		return
	}
	p.Release()*/
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Welcome to the CITIZEN SCIENCE COVID-19 %s\n", version)
	fmt.Println("to start press enter")
	reader.ReadString('\n')
	versionCheck()
	auto := false
	threads := int64(runtime.NumCPU())

	var save map[string]string
	data, err := ioutil.ReadFile("tmp_settings")
	fmt.Println(string(data))
	if err == nil {
		err = json.Unmarshal(data, &save)
		if err == nil {
			threadsC, ok := save["threads"]
			if ok {
				threadsN, err := strconv.ParseInt(threadsC, 10, 64)
				if err == nil {
					threads = threadsN
				}
			}

			autoC, ok := save["auto"]
			if ok {
				autoN, err := strconv.ParseBool(autoC)
				if err == nil {
					auto = autoN
				}
			}

		}
	}
	os.Remove("tmp_settings")

	if threads > 1 {
		fmt.Printf("Your computer is capable of using %d simultaneous threads, how many of them do you want to utilize?\n", threads)
		input, err := reader.ReadString('\n')
		input = strings.Replace(input, "\n", "", -1)
		input = strings.TrimSpace(input)
		newThreads, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			threads = threads / 2
			fmt.Printf("Error reading your input, utilizing half of available threads: %d\n", threads)
		}
		threads = newThreads
	}

	pCounter := 0
	for {
		if pCounter == 10 {
			autoString := "false"
			if auto {
				autoString = "true"
			}
			tempSettings := map[string]string{
				"threads": fmt.Sprintf("%d", threads),
				"auto":    autoString,
			}
			tmpJSON, err := json.Marshal(tempSettings)
			if err != nil {
				fmt.Println("Problem storing settings, will use default.")
			} else {
				ioutil.WriteFile("tmp_settings", tmpJSON, 0644)
			}

			versionCheck()
			pCounter = 0
		}
		pCounter++
		if !auto {
			fmt.Println("Do you want to continue automatically when done calculating a package? (y/n)")
			txt, _ := reader.ReadString('\n')
			if strings.Contains(txt, "y") {
				auto = true
			}
		}

		target, ok := getTarget(0)
		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		if target == -1 {
			fmt.Println("No new targets, waiting...")
			<-time.After(time.Minute * 30)
			continue
		}

		downloadPrerequiredFiles(0, int(target))
		fmt.Println("Prerequired files downloaded")
		counter, ok := getCounter(0, int(target))
		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		if counter == -1 {
			fmt.Println("No new packages, waiting...")
			<-time.After(time.Minute * 30)
			continue
		}
		fmt.Printf("Calculating package %d\n", counter)
		ok = getPackageFile(counter, 0, target)
		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		ok = startFlexing(counter, threads)
		if !ok {
			<-time.After(time.Second * 10)
			continue
		}
		fmt.Println("Done calculating page")
		ok = uploadFile(counter, 0, target)
		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		fmt.Println("File uploaded page")
	}

}

func uploadFile(number, count, target int64) bool {
	extraParams := map[string]string{
		"apikey": apikey,
	}
	request, err := newfileUploadRequest(fmt.Sprintf("%s/%d/file/%d", host, target, number), extraParams, "data", fmt.Sprintf("OUT_%d.sdf", number))
	if err != nil {
		if count == 5 {
			log.Println("Error preparing file for upload, aborting", err)
			return false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error preparing file for upload, retrying in 5 sec")
		return uploadFile(number, count+1, target)

	}
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		if count == 5 {
			log.Println("Error uploading file, aborting", err)
			return false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error uploading file, retrying in 5 sec")
		return uploadFile(number, count+1, target)

	}
	err = os.Remove(fmt.Sprintf("OUT_%d.sdf", number))
	if err != nil {
		fmt.Println("Error removing output file, continuing anyway...")
	}
	return true
}

// Creates a new file upload http request with optional extra params
func newfileUploadRequest(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

func startFlexing(number, threads int64) bool {
	//FLEXX\flexx -p TEST_PRO.pdb  -r TEST_REF.sdf -i %fx% -o %outfx% -v 4 --max-nof-conf 1
	command := fmt.Sprintf("lib\\FLEXX\\flexx -p TEST_PRO.pdb  -r TEST_REF.sdf -i 3D_structures_%d.sdf -o OUT_%d.sdf -v 4 --max-nof-conf 1 --thread-count %d", number, number, threads)
	fmt.Println(command)
	c := exec.Command("cmd", "/C", command)

	var stdBuffer bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &stdBuffer)

	c.Stdout = mw
	c.Stderr = mw

	// Execute the command
	if err := c.Run(); err != nil {
		fmt.Printf("Error flexing package (stdout) %d : %s\n", number, err)
		return false
	}

	log.Println(stdBuffer.String())
	return true
	/*stdout, err := c.StdoutPipe()
	if err != nil {
		fmt.Printf("Error flexing package (stdout) %d : %s\n", number, err)
		return
	}
	err = c.Start()
	if err != nil {
		fmt.Printf("Error flexing package (start) %d : %s\n", number, err)
		return
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		m := scanner.Text()
		fmt.Print(m)
	}

	err = c.Wait()
	if err != nil {
		fmt.Printf("Error flexing package (wait) %d : %s\n", number, err)
		return
	}?*/
}

func getPackageFile(number, count, target int64) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	response, err := client.Get(fmt.Sprintf("%s/%d/file/down/%d", host, target, number))
	if err != nil {
		if count == 5 {
			log.Panic("Error getting package file, aborting", err)
			return false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error getting package fil, retrying in 5 sec")
		return getPackageFile(number, count+1, target)

	}
	defer response.Body.Close()
	out, err := os.Create(fmt.Sprintf("3D_structures_%d.sdf", number))
	if err != nil {
		log.Println("Error storing package file, aborting", err)
		return false
	}
	defer out.Close()
	_, err = io.Copy(out, response.Body)
	if err != nil {
		log.Println("Error storing package file, aborting", err)
		return false
	}
	return true

}

func getCounter(count, target int) (int64, bool) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	response, err := client.Get(fmt.Sprintf("%s/%d/counter", host, target))
	if err != nil {
		if count == 5 {
			log.Println("Error getting counter, aborting", err)
			return 0, false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error getting counter, retrying in 5 sec")
		return getCounter(count+1, target)

	}
	defer response.Body.Close()
	stringBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		if count == 5 {
			log.Println("Error getting counter, aborting", err)
			return 0, false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error getting counter, retrying in 5 sec")
		return getCounter(count+1, target)

	}
	counter, err := strconv.ParseInt(string(stringBody), 10, 64)
	if err != nil {
		if count == 5 {
			log.Println("Error getting counter, aborting", err)
			return 0, false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error getting counter, retrying in 5 sec")
		return getCounter(count+1, target)

	}
	return counter, true
}

func downloadPrerequiredFiles(count, target int) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	response, err := client.Get(fmt.Sprintf("%s/%d/file/target/test_pro", host, target))
	if err != nil {
		if count == 5 {
			log.Panic("Error downloading files, aborting")
			return
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error downloading files, retrying in 5 sec")
		downloadPrerequiredFiles(count+1, target)
		return
	}
	defer response.Body.Close()
	out, err := os.Create("TEST_PRO.pdb")
	if err != nil {
		log.Panic("Error storing TEST_PRO.pdb file, aborting", err)
		return
	}
	defer out.Close()
	_, err = io.Copy(out, response.Body)
	if err != nil {
		log.Panic("Error storing TEST_PRO.pdb file, aborting", err)
		return
	}
	//http://localhost:8888/file/target/test_ref

	response2, err2 := client.Get(fmt.Sprintf("%s/%d/file/target/test_ref", host, target))
	if err2 != nil {
		if count == 5 {
			log.Panic("Error downloading files, aborting")
			return
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error downloading files, retrying in 5 sec")
		downloadPrerequiredFiles(count+1, target)
		return
	}
	defer response2.Body.Close()
	out2, err2 := os.Create("TEST_REF.sdf")
	if err2 != nil {
		log.Panic("Error storing TEST_REF.sdf file, aborting", err2)
		return
	}
	defer out2.Close()
	_, err2 = io.Copy(out2, response2.Body)
	if err2 != nil {
		log.Panic("Error storing TEST_REF.sdf file, aborting", err2)
		return
	}

}

func getTarget(count int) (int64, bool) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	response, err := client.Get(fmt.Sprintf("%s/target", host))
	if err != nil {
		if count == 5 {
			log.Println("Error getting target, aborting", err)
			return 0, false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error getting target, retrying in 5 sec")
		return getTarget(count + 1)

	}
	defer response.Body.Close()
	stringBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		if count == 5 {
			log.Println("Error getting target, aborting", err)
			return 0, false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error getting target, retrying in 5 sec")
		return getTarget(count + 1)

	}
	counter, err := strconv.ParseInt(string(stringBody), 10, 64)
	if err != nil {
		if count == 5 {
			log.Println("Error getting target, aborting", err)
			return 0, false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error getting target, retrying in 5 sec")
		return getTarget(count + 1)

	}
	return counter, true
}
