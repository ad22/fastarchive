# fastarchive: tar+gzip over ssh in golang
This utility provides a go implementation of tar over SSH. It uses the io.Writer and io.Reader interfaces
that are chained together much like *nix pipes, resulting in no intermediate buffering or save. <br>

The use case for this was to support file transfer in a large CI/CD deployment, where 1000's of files
were required to be compressed, as well as transferred individually to an archive server, on multiple jobs.

#### Description<br>
This tool generates a stream of tar+gz over an SSH pipe, as well as writes to a compressed zip and/or 
tar archive (if selected), in a single pass of the given directory. It then transfer the zip and tar
over the same SSH connection<br>

#### Example<br>
    time ./fastarchive -server archive-backend-01 -destpath sshtest -user server -createzip -zipname "build-a.zip" -createtargz -targzname "build-a.tar.gz" archive
    path archive streamed to 3 writer(s)
    path build-a.zip streamed to 1 writer(s)
    path build-a.tar.gz streamed to 1 writer(s)
    done.

    real    1m23.797s
    user    0m36.495s
    sys     0m8.477s


## Performance
The test transfers about 830M of data with 45000 small files to a remote server over the internet.

#### fastarchive
    time ./fastarchive -server archive-backend-01 -user server -destpath sshtest test-files/
    path test-files streamed to 1 writer(s)
    done.

    real	0m50.895s
    user	0m8.599s
    sys	0m11.586s


#### OSX tar + ssh
    time tar -czf - test-files/ | ssh server@archive-backend-01 "tar -xzf - -C sshtest"

    real	0m58.359s
    user	0m29.945s
    sys	0m12.309s

#### OSX rsync with compression
    time rsync -az test-files/ server@archive-backend-01:sshtest/

    real	0m54.740s
    user	0m21.290s
    sys	0m14.965s

#### scp (no compression)
    time scp -r test-files/ server@archive-backend-01:sshtest/

    10 minutes and still running, I got bored :-)

<br>

####Notes:
This utility has switched to using github.com/mholt/archive which was found to be more flexible to
switch compression types easily.
