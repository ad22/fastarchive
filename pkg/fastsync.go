package main

import (
	"archive/tar"
	"bufio"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var fromFile, basePath, destPath, sshServer, sshUser, sshKeyPath, knownHostsFile string
var noVerify bool
var port int
var paths []string

func init() {
	usr, err := user.Current()
	if err != nil {
		log.Fatalln(err)
	}

	flag.StringVar(&fromFile, "fromfile", "", "Specify paths to archive via a file, one path per line")
	flag.StringVar(&basePath, "basepath", "", "Strip off the base path from all paths and transfer " +
		"relative to this path. Note that any leading / will be stripped by default")
	flag.StringVar(&sshUser, "sshuser", "root", "SSH user")
	flag.StringVar(&sshServer, "sshserver", "", "SSH server to archive to")
	flag.IntVar(&port, "sshport", 22, "SSH port to use")
	flag.StringVar(&sshKeyPath, "sshkeypath", filepath.Join(usr.HomeDir, ".ssh", "id_rsa"), "SSH Private key to authenticate against the server")
	flag.BoolVar(&noVerify, "sshnoverify", false, "Do not verify SSH host key")
	flag.StringVar(&knownHostsFile, "sshknownhostspath", filepath.Join(usr.HomeDir, ".ssh", "known_hosts"), "SSH Known hosts file")
	flag.StringVar(&destPath, "destpath", "", "Destination path to archive to")
	flag.Parse()
	paths = flag.Args()
	if sshServer == "" {
		flag.Usage()
		log.Fatalln("argument -sshserver is required")
	}
	if destPath == "" {
		flag.Usage()
		log.Fatalln("argument -destpath is required. Use . to indicate destination as the default homedir on remote")
	}
	if fromFile == "" && len(paths) == 0 {
		flag.Usage()
		log.Fatalln("either -fromfile or space separated file/dir paths are required")
	}
	if fromFile != "" && len(paths) != 0 {
		flag.Usage()
		log.Fatalln("only one of the options: -fromfile or space separated file paths is allowed")
	}
	if fromFile != "" {
		paths, err = readLinesFromFile(fromFile); if err != nil {
			log.Fatalln(err)
		}
	}
}

func main() {
	session, err := createSSHSession(sshUser, sshServer, port, sshKeyPath, knownHostsFile, noVerify)
	errs := make(chan error)
	finished := make(chan bool, 1)
	if err != nil {
		log.Fatalln(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go createAndExtract(session, destPath, &wg, errs)
	go writeArchivesRecurse(paths, basePath, session, &wg, errs)
	go func() {
		wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
	case err := <-errs:
		close(errs)
		log.Fatalln("error: ", err)
		return
	}
	fmt.Println("successfully uploaded")
}

func readLinesFromFile(path string) ([]string, error) {
	var lines []string
	file, err := os.Open(path)
	if err != nil {
		return lines, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return lines, err
	}
	return lines, nil
}

func cleanAndTrimForTar(path string, basePath string) string {
	newPath := filepath.Clean(path)
	basePath = filepath.Clean(basePath)
	if basePath != "" {
		if strings.HasPrefix(newPath, basePath) {
			newPath = strings.TrimPrefix(newPath, basePath)
		}
	}
	volumeName := filepath.VolumeName(newPath)
	if volumeName != "" {
		newPath = strings.TrimLeft(newPath, volumeName)
	}
	newPath = filepath.ToSlash(newPath)
	newPath = strings.TrimLeft(newPath, "/")
	return newPath
}

func writeTarHeaders(tw *tar.Writer, info os.FileInfo, path string, newPath string) error {
	fmt.Printf("%v -> %v\n", path, newPath)
	if h, err := tar.FileInfoHeader(info, newPath); err != nil {
		return err
	} else {
		h.Name = newPath
		if err = tw.WriteHeader(h); err != nil {
			return err
		}
	}
	return nil
}

func writeArchivesRecurse(srcPaths []string, basePath string, session *ssh.Session, wg *sync.WaitGroup, errs chan <-error) {
	defer wg.Done()
	defer session.Close()
	stdin, err := session.StdinPipe()
	if err != nil {
		errs <- err
		return
	}
	defer stdin.Close()
	tw := tar.NewWriter(stdin)
	defer tw.Close()

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsDir() {
			return nil
		}
		if len(path) == 0 {
			return nil
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()
		newPath := cleanAndTrimForTar(path, basePath)
		if err := writeTarHeaders(tw, info, path, newPath); err != nil {
			return err
		}
		if _, err := io.Copy(tw, fr); err != nil {
			return err
		}
		return nil
	}

	for _, subPath := range srcPaths {
		if err := filepath.Walk(subPath, walkFn); err != nil {
			errs <- err
		}
	}
}

func createAndExtract(session *ssh.Session, destPath string, wg *sync.WaitGroup, errs chan <-error) {
	defer wg.Done()
	defer session.Wait()
	err := session.Start("tar -xf - -C " + destPath)
	if err != nil {
		errs <- err
	}
}

func createSSHSession(user, server string, port int, sshKeyPath, knownHostsFile string, noVerify bool) (*ssh.Session, error) {
	pKey, err := ioutil.ReadFile(sshKeyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("ssh key path %v: ErrNotExist", sshKeyPath)
		} else if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("ssh key path %v: ErrPermission", sshKeyPath)
		}
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pKey);
	if err != nil {
		return nil, fmt.Errorf("could not parse private key %v", sshKeyPath)
	}
	var hostKeyCallBack ssh.HostKeyCallback
	if noVerify {
		hostKeyCallBack = ssh.InsecureIgnoreHostKey()
	} else if knownHostsFile != "" {
		hostKeyCallBack, err = knownhosts.New(knownHostsFile);
		if err != nil {
			return nil, fmt.Errorf("could not create hostkeycallback function: %v", err)
		}
	} else {
		hostKeyCallBack, err = knownhosts.New()
		if err != nil {
			return nil, fmt.Errorf("could not create hostkeycallback function: %v", err)
		}
	}
	clientConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallBack,
		Timeout:         time.Minute,
	}
	client, err := ssh.Dial("tcp", server+ ":" + strconv.Itoa(port), clientConfig)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	return session, nil
}
