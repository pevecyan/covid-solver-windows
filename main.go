package main

import (
	"archive/zip"
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
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	host    = "<apiserver>"
	apikey  = "<apikey>"
	version = "V1.9"
	debug   = false
)

func versionCheck() {
	type versionCheckData struct {
		TagName string `json:"tag_name"`
	}

	client := &http.Client{
		Timeout: time.Second * 20,
	}

	response, err := client.Get("https://api.github.com/repos/pevecyan/covid-solver-windows/releases/latest")
	if err != nil {
		fmt.Println("Error fecthing latest version, using old")
		return
	}
	defer response.Body.Close()
	var versionData versionCheckData
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&versionData)
	if err != nil {
		fmt.Println("Problem fetching latest release version")
		return
	}
	if version != versionData.TagName {
		fmt.Println("----------------------------------------------------------------------")
		fmt.Println("------------------------NEW VERSION AVAILABLE-------------------------")
		fmt.Println("---https://github.com/pevecyan/covid-solver-windows/releases/latest---")
		fmt.Println("----------------------------------------------------------------------")

	}

}

func SetupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGABRT)
	signal.Notify(c, os.Interrupt, syscall.SIGKILL)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	signal.Notify(c, os.Interrupt, syscall.SIGQUIT)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		os.Exit(0)
	}()
}

func main() {
	SetupCloseHandler()
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Welcome to the COVID Solver (Windows) %s\n", version)
	versionCheck()
	fmt.Println(`# Copyright Notice and Disclaimer
# ===============================
#
# This software (COVID Solver, win, linux, mac versions) along with its
# source code is released under the terms of the 
# GNU General Public License version 3 (GPL v3).
# A full copy of the GNU General Public License can be found at:
#  <https://www.gnu.org/licenses/>.
#
#
# Permission to use, copy, modify and distribute
# versions of this software and its documentation for any purpose and
# without fee is hereby granted, provided that the above copyright
# notice appear in all copies and that both the copyright notice and
# this permission notice appear in supporting documentation, and that
# the name(s) of the author(s) not be used in advertising or publicity
# pertaining to distribution of the software without specific, written
# prior permission.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# THE AUTHORS (Žan Pevec, Gašper Tomšič, Marko Jukić, Črtomir Podlipnik
# and supporting organisations at the COVID.si project - www.covid.si)
# DISCLAIM ALL WARRANTIES WITH REGARD TO THIS SOFTWARE,
# INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS.  IN
# NO EVENT SHALL THE AUTHOR(S) BE LIABLE FOR ANY SPECIAL, INDIRECT OR
# CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF
# USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
# OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
# PERFORMANCE OF THIS SOFTWARE.
# ----------------------------------------------------------------------
#
# If You run this software you read the licence in full conscience and agree
# to the terms described above.`)
	fmt.Println("To agree and continue press enter")
	if !debug {
		reader.ReadString('\n')
	}
	//versionCheck()
	fmt.Println("-------------------------")
	fmt.Println("For this software to work you need Microsoft Visual C++ 2010 Redistributable Package installed.")
	fmt.Println("Have you already installed it? y/n")
	vcredist, _ := reader.ReadString('\n')
	vcredist = strings.Replace(vcredist, "\n", "", -1)
	vcredist = strings.TrimSpace(vcredist)
	vcredist = strings.ToLower(vcredist)
	if vcredist != "y" && vcredist != "yes" {
		cmd := exec.Command("VC_redist.x64.exe")
		cmd.Run()
		os.Exit(0)

	}

	auto := false
	threads := int64(runtime.NumCPU())

	/*var save map[string]string
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
	*/
	if threads > 1 {
		fmt.Printf("Your computer is capable of using %d simultaneous threads, how many of them do you want to utilize?\n", threads)
		input := "4"
		var err error
		if !debug {
			input, err = reader.ReadString('\n')
		}
		input = strings.Replace(input, "\n", "", -1)
		input = strings.TrimSpace(input)
		newThreads, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			threads = threads / 2
			fmt.Printf("Error reading your input, utilizing half of available threads: %d\n", threads)
		} else {
			threads = newThreads
		}
	}

	for {
		versionCheck()

		if !auto {
			fmt.Println("Do you want to continue automatically when done calculating a package? (y/n)")
			txt := "y"
			if !debug {
				txt, _ = reader.ReadString('\n')
			}

			if strings.Contains(txt, "y") {
				auto = true
			}
		}

		os.RemoveAll("output")
		os.RemoveAll("package")
		os.RemoveAll("target")
		os.RemoveAll("log")

		target, ok := getTarget(0)
		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		if target == -1 {
			fmt.Println("No new targets, waiting...")
			<-time.After(time.Minute * 3)
			continue
		}

		ok = downloadPrerequiredFiles(0, int(target))

		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		fmt.Println("Prerequired files downloaded")
		_, err := Unzip(fmt.Sprintf("TARGET_%d.zip", target), "target")
		if err != nil {
			fmt.Println("Error decompressing target file")
			os.RemoveAll("target")
			os.Remove(fmt.Sprintf("TARGET_%d.zip", target)) //TODO put it back
			<-time.After(time.Second * 10)
			continue
		}

		os.Remove(fmt.Sprintf("TARGET_%d.zip", target)) //TODO put it back
		//Fix files
		data, err := ioutil.ReadFile(fmt.Sprintf("target/TARGET_%d.prm", target))
		if err != nil {
			fmt.Println("Error preparing target file")
			<-time.After(time.Second * 10)
			continue
		}

		stringData := strings.ReplaceAll(string(data), "RECEPTOR_FILE TARGET_PRO_", "RECEPTOR_FILE target/TARGET_PRO_")
		stringData = strings.ReplaceAll(stringData, "REF_MOL TARGET_REF_", "REF_MOL target/TARGET_REF_")
		err = ioutil.WriteFile(fmt.Sprintf("target/TARGET_%d.prm", target), []byte(stringData), 0777)
		if err != nil {
			fmt.Println("Error preparing target file, step 2")
			<-time.After(time.Second * 10)
			continue
		}

		counter, ok := getCounter(0, int(target))
		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		if counter == -1 {
			fmt.Println("No new packages, waiting...")
			<-time.After(time.Minute * 3)
			continue
		}
		fmt.Printf("Calculating package %d\n", counter)

		ok = getPackageFile(counter, 0, target)

		if !ok {
			<-time.After(time.Second * 30)
			continue
		}

		ok = splitPackage(counter, threads)
		if !ok {
			fmt.Println("Problem splitting package")
			<-time.After(time.Second * 10)
		}
		ok = startDocking(counter, threads, target)
		if !ok {
			<-time.After(time.Second * 10)
			continue
		}
		ok = joinPackage(counter, target, threads)
		if !ok {
			fmt.Println("Problem joining output")
			<-time.After(time.Second * 10)
		}

		fmt.Println("Done calculating page")
		ok = uploadFile(counter, 0, target)
		if !ok {
			<-time.After(time.Second * 30)
			continue
		}
		fmt.Println("File uploaded page")

		//CLEAN UP

	}

}

func joinPackage(count, target, threads int64) bool {
	files, err := ioutil.ReadDir("package")
	if err != nil {
		fmt.Println("Error reading package directory")
		return false
	}
	var data string
	for _, file := range files {
		if strings.Contains(file.Name(), "_out.sd") {
			fileData, err := ioutil.ReadFile(fmt.Sprintf("package/%s", file.Name()))
			if err != nil {
				fmt.Println("Error reading package output file")
				return false
			}
			data += string(fileData)
		}
	}
	os.Mkdir("output", 0777)
	err = ioutil.WriteFile(fmt.Sprintf("output/OUT_T%d_%d.sdf", target, count), []byte(data), 0777)
	if err != nil {
		fmt.Println("Error writing result to file")
		return false
	}
	return true
}

func splitPackage(counter, threads int64) bool {
	data, err := ioutil.ReadFile(fmt.Sprintf("3D_structures_%d.sdf", counter))
	if err != nil {
		fmt.Println("Error reading package data")
		return false
	}
	stringData := string(data)
	splitted := strings.Split(stringData, "$$$$")
	fileSize := len(splitted) / int(threads)
	lastFileSize := len(splitted) % int(threads)

	currentPosition := 0
	os.Mkdir("package", 0777)
	for i := 0; i < int(threads); i++ {
		fileData := ""
		if i == int(threads)-1 {
			fileData = strings.Join(splitted[currentPosition:], "$$$$")
		} else {
			fileData = strings.Join(splitted[currentPosition:currentPosition+fileSize], "$$$$")
		}
		currentPosition += fileSize
		ioutil.WriteFile(fmt.Sprintf("package/package_%d.sdf", i), []byte(fileData), 0777)
	}
	fmt.Println(fileSize, lastFileSize)
	os.Remove(fmt.Sprintf("3D_structures_%d.sdf", counter))
	return true
}

func uploadFile(number, count, target int64) bool {
	extraParams := map[string]string{
		"apikey": apikey,
	}
	request, err := newfileUploadRequest(fmt.Sprintf("%s/%d/file/%d", host, target, number), extraParams, "data", fmt.Sprintf("output/OUT_T%d_%d.sdf", target, number))
	if err != nil {
		if count == 5 {
			log.Println("Error preparing file for upload, aborting", err)
			return false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error preparing file for upload, retrying in 5 sec")
		return uploadFile(number, count+1, target)

	}
	client := &http.Client{
		Timeout: time.Second * 120,
	}
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

func startDocking(number, threads, target int64) bool {
	running := int(threads)
	done := make(chan bool)
	os.Mkdir("log", 0777)

	for i := 0; i < int(threads); i++ {
		go func(count int) {
			command := fmt.Sprintf("lib\\rxdock\\builddir-win64\\rbdock.exe -r target\\TARGET_%d.prm -p dock.prm -f \\target\\htvs.ptc -i package\\package_%d.sdf -o package\\package_%d_out", target, count, count)
			fmt.Println(command)
			splited := strings.Split(command, " ")
			c := exec.Command(splited[0], splited[1:]...)
			var stdBuffer bytes.Buffer
			file, _ := os.Create(fmt.Sprintf("log/rxdock_out_%d", count))
			if debug {
				mw := io.MultiWriter(os.Stdout, &stdBuffer)

				c.Stdout = mw
				c.Stderr = mw

			} else {

				c.Stdout = file
				c.Stderr = file

			}

			// Execute the command
			if err := c.Run(); err != nil {
				exitError, ok := err.(*exec.ExitError)
				if ok {
					fmt.Printf("Error docking package (stdout) %d : %d\n", count, exitError.ExitCode())
					if exitError.ExitCode() == 3221225786 {
						os.Exit(0)
					}
				} else {
					fmt.Println(err)
				}
				file.Close()

				done <- false
				return
			}
			file.Close()
			done <- true
			if debug {
				log.Println(stdBuffer.String())
			}
		}(i)
	}

	for {
		select {
		case ok := <-done:
			{
				if ok {

				}
				running--
				if running == 0 {
					fmt.Println("Package done calculating")
					return true
				}

			}
		}
	}
}

func getPackageFile(number, count, target int64) bool {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	extraParams, _ := json.Marshal(map[string]string{
		"apikey": apikey,
	})
	response, err := client.Post(fmt.Sprintf("%s/%d/file/down/%d", host, target, number), "application/json", bytes.NewBuffer(extraParams))
	if err != nil {
		if count == 5 {
			log.Println("Error getting package file, aborting", err)
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
		Timeout: 60 * time.Second,
	}
	extraParams, _ := json.Marshal(map[string]string{
		"apikey": apikey,
	})

	response, err := client.Post(fmt.Sprintf("%s/%d/counter", host, target), "application/json", bytes.NewBuffer(extraParams))
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

func downloadPrerequiredFiles(count, target int) bool {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	extraParams, _ := json.Marshal(map[string]string{
		"apikey": apikey,
	})

	response, err := client.Post(fmt.Sprintf("%s/%d/file/target/archive", host, target), "application/json", bytes.NewBuffer(extraParams))
	if err != nil {
		if count == 5 {
			log.Println("Error downloading files, aborting")
			return false
		}
		<-time.After(time.Second * 5)
		fmt.Println("Error downloading files, retrying in 5 sec")
		return downloadPrerequiredFiles(count+1, target)
	}
	defer response.Body.Close()
	out, err := os.Create(fmt.Sprintf("TARGET_%d.zip", target))
	if err != nil {
		log.Println("Error storing TARGET_.zip file, aborting", err)
		return false
	}
	defer out.Close()
	_, err = io.Copy(out, response.Body)
	if err != nil {
		log.Println("Error storing TARGET_.zip file, aborting", err)
		return false
	}
	return true

}

func getTarget(count int) (int64, bool) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	extraParams, _ := json.Marshal(map[string]string{
		"apikey": apikey,
	})

	response, err := client.Post(fmt.Sprintf("%s/target", host), "application/json", bytes.NewBuffer(extraParams))
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

// Unzip will decompress a zip archive, moving all files and folders
// within the zip file (parameter 1) to an output directory (parameter 2).
func Unzip(src string, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return filenames, err
		}

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return filenames, err
		}
	}
	return filenames, nil
}
