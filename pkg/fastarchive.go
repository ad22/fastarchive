package main

import (
	"flag"
	"fmt"
	"github.com/mholt/archiver/v3"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sync"
)

var fromFile, destPath, server, userName, sshKeyPath, knownHostsFile, createLocalZipFileName, createLocalTarGzFileName string
var noVerify, createLocalZip, createLocalTarGz bool
var port int
var paths []string

func init() {
	usr, err := user.Current()
	if err != nil {
		log.Fatalln(err)
	}

	flag.StringVar(&fromFile, "fromfile", "", "Specify paths to archive via a file, one path per line")
	flag.StringVar(&destPath, "destpath", "", "Destination path to archive to")

	flag.StringVar(&server, "server", "", "SSH server to archive to (SSH or artifactory HTTP)")
	flag.IntVar(&port, "port", 22, "Server port to use (SSH or artifactory)")
	flag.StringVar(&userName, "user", "root", "Username (SSH or artifactory)")

	flag.StringVar(&sshKeyPath, "sshkeypath", filepath.Join(usr.HomeDir, ".ssh", "id_rsa"), "SSH Private key to authenticate against the server")
	flag.BoolVar(&noVerify, "sshnoverify", false, "Do not verify SSH host key")
	flag.StringVar(&knownHostsFile, "sshknownhostspath", filepath.Join(usr.HomeDir, ".ssh", "known_hosts"), "SSH Known hosts file")

	flag.BoolVar(&createLocalZip, "createzip", false, "Create a zip file in the current working directly with all contents streamed, and archive it alongside")
	flag.BoolVar(&createLocalTarGz, "createtargz", false, "Create a tar gz file in the current working directly with all contents streamed, and archive it alongside")
	flag.StringVar(&createLocalZipFileName, "zipname", "", "Name of zip file created when -createzip is used")
	flag.StringVar(&createLocalTarGzFileName, "targzname", "", "Name of tar gz file created when -createtargz is used")

	flag.Parse()
	paths = flag.Args()
	if server == "" {
		flag.Usage()
		log.Fatalln("argument -server is required")
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

	if createLocalZip && createLocalZipFileName == "" {
		log.Fatalln("-zipname is required when -createzip is specified")
	}
	if createLocalTarGz && createLocalTarGzFileName == "" {
		log.Fatalln("-targzname is required when -createtargz is specified")
	}
}

func main() {
	session, err := createSSHSession(userName, server, port, sshKeyPath, knownHostsFile, noVerify)
	if err != nil {
		log.Fatalln(err)
	}
	defer session.Close()

	// SSH goroutine wg
	wgErrs := make(chan error)
	wgFinished := make(chan bool, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	// stream goroutine wg
	swgErrs := make(chan error)
	swgFinished := make(chan bool, 1)
	var swg sync.WaitGroup
	swg.Add(1)

	stdinPipe, err := sshStdinPipe(session)
	if err != nil {
		log.Fatalln(err)
	}

	var writers []archiver.Writer
	tw, err := generateTarGzWriter(*stdinPipe)
	if err != nil {
		log.Fatalln(err)
	}
	writers = append(writers, tw)

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	var z *archiver.Zip
	if createLocalZip {
		zp := path.Join(wd, createLocalZipFileName)
		zf, err := os.Create(zp)
		if err != nil {
			log.Fatalln(err)
		}
		defer zf.Close()

		z, err = generateZipWriter(WriteFakeCloser{zf})
		if err != nil {
			log.Fatalln(err)
		}
		writers = append(writers, z)
	}

	var tfw *archiver.TarGz
	if createLocalTarGz {
		tp := path.Join(wd, createLocalTarGzFileName)
		tf, err := os.Create(tp)
		if err != nil {
			log.Fatalln(err)
		}
		defer tf.Close()
		tfw, err = generateTarGzWriter(WriteFakeCloser{tf})
		if err != nil {
			log.Fatalln(err)
		}
		writers = append(writers, tfw)
	}

	serverExtractCommand := "tar -xzf - -C " + destPath
	go sshCommandWait(serverExtractCommand, session, &wg, wgErrs)
	go walkAndStream(paths, writers, &swg, swgErrs, false, nil)

	err = processWg(&swg, swgFinished, swgErrs)
	tfw.Close()
	z.Close()
	if err != nil {
		tw.Close()
		log.Fatalln(err)
	}

	var finalPaths []string
	tw.CompressionLevel = 0
	finalWriters := []archiver.Writer{tw}
	if createLocalZip || createLocalTarGz {
		wg.Add(1)
		if createLocalZip {
			finalPaths = append(finalPaths, createLocalZipFileName)
		}
		if createLocalTarGz {
			finalPaths = append(finalPaths, createLocalTarGzFileName)
		}
		go walkAndStream(finalPaths, finalWriters, &wg, wgErrs, true, *stdinPipe)
	} else {
		tw.Close()
	}

	err = processWg(&wg, wgFinished, wgErrs)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("done.")
}
