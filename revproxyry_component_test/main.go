package main

// tests the revproxyry as a component.

import (
	"os"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"net/http"
	"time"

	"github.com/phayes/freeport"
)

// testListDir tests that the directory is correctly listed.
func testListDir(revproxyBinary string) error {
	fmt.Println("Running testListDir ...")

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create a temporary directory: %s", err.Error())
	}
	defer os.RemoveAll(testDir)

	func() {
		pth := filepath.Join(testDir, "some-file.txt")
		f, err := os.Create(pth)
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()

		f.Write([]byte("hello"))
	}()

	port, err := freeport.GetFreePort()
	if err != nil {
		return fmt.Errorf("failed to acquire a free port: %s", err.Error())
	}

	cfgTxt := fmt.Sprintf(`
{
  "domain": "",
  "ssl_key_path": "",
  "letsencrypt_dir": "",
  "https_address": "",
  "http_address": ":%d",
  "ssl_cert_path": "",
  "routes": [
    {
      "prefix": "/o/",
      "target": "%s",
      "auths": ["anonymous"]
    }
  ],
  "auths": {
    "anonymous": {
      "username": "",
      "password_hash": ""
    }
  }
}`, port, testDir)

	cfgPth := filepath.Join(testDir, "config.json")
	func() {
		f, err := os.Create(cfgPth)
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()

		f.Write([]byte(cfgTxt))
	}()

	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}

	proc, err := os.StartProcess(
		revproxyBinary,
		[]string{revproxyBinary, "-config_path", cfgPth},
		&os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})

	if err != nil {
		return fmt.Errorf("failed to start the process: %s", err.Error())
	}

	exited := false
	defer func() {
		if !exited {
			proc.Kill()
		}
	}()

	fmt.Println("Sleeping to allow the server to start...")
	time.Sleep(3 * time.Second)

	url := fmt.Sprintf("http://@127.0.0.1:%d/o/", port)

	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch the directory listing: %s", err.Error())
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read the body: %s", err.Error())
	}

	content := string(data)

	expectedContent := "<pre>\n<a href=\"config.json\">config.json</a>\n" +
		"<a href=\"some-file.txt\">some-file.txt</a>\n</pre>\n"

	if content != expectedContent {
		fmt.Fprintf(os.Stderr, "Expected contents %#v, got %#v", expectedContent, content)
	}

	return nil
}

func testMD5(revproxyBinary string) error {
	fmt.Println("Running testMD5 ...")

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create a temporary directory: %s", err.Error())
	}
	defer os.RemoveAll(testDir)

	port, err := freeport.GetFreePort()
	if err != nil {
		return fmt.Errorf("failed to acquire a free port: %s", err.Error())
	}

	cfgTxt := fmt.Sprintf(`
{
  "domain": "",
  "ssl_key_path": "",
  "letsencrypt_dir": "",
  "https_address": "",
  "http_address": ":%d",
  "ssl_cert_path": "",
  "routes": [
    {
      "prefix": "/o/",
      "target": "%s",
      "auths": ["some-user"]
    }
  ],
  "auths": {
    "some-user": {
      "username": "some-user",
      "password_hash": "$apr1$cVKAnC1K$wWAv8sB0n8iKuFkhaMI0a."
    }
  }
}`, port, testDir)

	cfgPth := filepath.Join(testDir, "config.json")
	func() {
		f, err := os.Create(cfgPth)
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()

		f.Write([]byte(cfgTxt))
	}()

	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}

	proc, err := os.StartProcess(
		revproxyBinary,
		[]string{revproxyBinary, "-config_path", cfgPth},
		&os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})

	if err != nil {
		return fmt.Errorf("failed to start the process: %s", err.Error())
	}

	exited := false
	defer func() {
		if !exited {
			proc.Kill()
		}
	}()

	fmt.Println("Sleeping to allow the server to start...")
	time.Sleep(3 * time.Second)

	// succeeds
	err = func() error {
		url := fmt.Sprintf("http://some-user:some-password@127.0.0.1:%d/o/", port)

		response, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to fetch the directory listing: %s", err.Error())
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("expected status code %d, but got: %d", http.StatusOK, response.StatusCode)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	// fails
	err = func() error {
		url := fmt.Sprintf("http://some-user:invalid-password@127.0.0.1:%d/o/", port)

		response, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to fetch the directory listing: %s", err.Error())
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			return fmt.Errorf("expected status code %d, but got: %d",
				http.StatusUnauthorized, response.StatusCode)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	return nil
}

func testBcryptRevproxyhashry(revproxyBinary string) error {
	fmt.Println("Running testBcryptRevproxyhashry ...")

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create a temporary directory: %s", err.Error())
	}
	defer os.RemoveAll(testDir)

	port, err := freeport.GetFreePort()
	if err != nil {
		return fmt.Errorf("failed to acquire a free port: %s", err.Error())
	}

	cfgTxt := fmt.Sprintf(`
{
  "domain": "",
  "ssl_key_path": "",
  "letsencrypt_dir": "",
  "https_address": "",
  "http_address": ":%d",
  "ssl_cert_path": "",
  "routes": [
    {
      "prefix": "/o/",
      "target": "%s",
      "auths": ["some-user"]
    }
  ],
  "auths": {
    "some-user": {
      "username": "some-user",
      "password_hash": "$2a$12$IufPB.BMcVdI6UN1Lu/nrOaTTWBJvvaZoHhPRmno5OrbY6L8wKpWO"
    }
  }
}`, port, testDir)

	cfgPth := filepath.Join(testDir, "config.json")
	func() {
		f, err := os.Create(cfgPth)
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()

		f.Write([]byte(cfgTxt))
	}()

	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}

	proc, err := os.StartProcess(
		revproxyBinary,
		[]string{revproxyBinary, "-config_path", cfgPth},
		&os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})

	if err != nil {
		return fmt.Errorf("failed to start the process: %s", err.Error())
	}

	exited := false
	defer func() {
		if !exited {
			proc.Kill()
		}
	}()

	fmt.Println("Sleeping to allow the server to start...")
	time.Sleep(3 * time.Second)

	// succeeds
	err = func() error {
		url := fmt.Sprintf("http://some-user:some-password@127.0.0.1:%d/o/", port)

		response, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to fetch the directory listing: %s", err.Error())
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("expected status code %d, but got: %d", http.StatusOK, response.StatusCode)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	// fails
	err = func() error {
		url := fmt.Sprintf("http://some-user:invalid-password@127.0.0.1:%d/o/", port)

		response, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to fetch the directory listing: %s", err.Error())
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			return fmt.Errorf("expected status code %d, but got: %d",
				http.StatusUnauthorized, response.StatusCode)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	return nil
}

// testBcryptHtpasswd tests that the bcrypt password from Apache's htpasswd utility also work.
func testBcryptHtpasswd(revproxyBinary string) error {
	fmt.Println("Running testBcryptHtpasswd ...")

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create a temporary directory: %s", err.Error())
	}
	defer os.RemoveAll(testDir)

	port, err := freeport.GetFreePort()
	if err != nil {
		return fmt.Errorf("failed to acquire a free port: %s", err.Error())
	}

	cfgTxt := fmt.Sprintf(`
{
  "domain": "",
  "ssl_key_path": "",
  "letsencrypt_dir": "",
  "https_address": "",
  "http_address": ":%d",
  "ssl_cert_path": "",
  "routes": [
    {
      "prefix": "/o/",
      "target": "%s",
      "auths": ["some-user"]
    }
  ],
  "auths": {
    "some-user": {
      "username": "some-user",
      "password_hash": "$2y$10$iO/phN2PYP9kMTPmCrp/vOMjD5FI6yKqoJ/JzNjdwgJbGyL/RG07m"
    }
  }
}`, port, testDir)

	cfgPth := filepath.Join(testDir, "config.json")
	func() {
		f, err := os.Create(cfgPth)
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()

		f.Write([]byte(cfgTxt))
	}()

	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}

	proc, err := os.StartProcess(
		revproxyBinary,
		[]string{revproxyBinary, "-config_path", cfgPth},
		&os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})

	if err != nil {
		return fmt.Errorf("failed to start the process: %s", err.Error())
	}

	exited := false
	defer func() {
		if !exited {
			proc.Kill()
		}
	}()

	fmt.Println("Sleeping to allow the server to start...")
	time.Sleep(3 * time.Second)

	// succeeds
	err = func() error {
		url := fmt.Sprintf("http://some-user:some-password@127.0.0.1:%d/o/", port)

		response, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to fetch the directory listing: %s", err.Error())
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("expected status code %d, but got: %d", http.StatusOK, response.StatusCode)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	// fails
	err = func() error {
		url := fmt.Sprintf("http://some-user:invalid-password@127.0.0.1:%d/o/", port)

		response, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to fetch the directory listing: %s", err.Error())
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			return fmt.Errorf("expected status code %d, but got: %d",
				http.StatusUnauthorized, response.StatusCode)
		}

		return nil
	}()
	if err != nil {
		return err
	}

	return nil
}

func run() int {
	revproxyryBinary := flag.String("revproxyry_binary", "",
		"Path to the revproxyry executable binary")

	flag.Parse()

	if *revproxyryBinary == "" {
		fmt.Fprintf(os.Stderr, "-revproxyry_binary is mandatory.")
		flag.PrintDefaults()
		return 1
	}

	err := testListDir(*revproxyryBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testListDir failed: %s\n", err.Error())
		return 1
	}

	err = testMD5(*revproxyryBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testMD5 failed: %s\n", err.Error())
		return 1
	}

	err = testBcryptRevproxyhashry(*revproxyryBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testBcryptRevproxyhashry failed: %s\n", err.Error())
		return 1
	}

	err = testBcryptHtpasswd(*revproxyryBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testBcryptHtpasswd failed: %s\n", err.Error())
		return 1
	}

	return 0
}

func main() {
	os.Exit(run())
}
