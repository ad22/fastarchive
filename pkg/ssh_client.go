package main

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"
)

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


func sshCommandWait(command string, session *ssh.Session, wg *sync.WaitGroup, errs chan <-error) {
	defer wg.Done()
	defer session.Wait()
	err := session.Start(command)
	if err != nil {
		errs <- err
	}
}

func sshStdinPipe(session *ssh.Session) (*io.WriteCloser, error) {
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}
	return &stdin, err
}