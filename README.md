# fastsync: tar + ssh in golang
This utility provides a go implementation of tar over SSH. It uses the io.Writer and io.Reader interfaces
that are chained together much like *nix pipes, resulting in no intermediate buffering or save. <br>

Piping a tar stream over SSH is an efficient way to transfer data when CPU cycles
are not restricted, especially for moving large file trees.

Equivalent to
`tar -cf - <dir> | ssh server "tar -xf - -C <destdir>"`

## Performance
The test transfers about 830M of data with 45000 small files to a remote server in a data center (not local)
#### fastsync
    time ./fastsync -sshserver essd-archive-backend-01 -destpath sshtest --sshuser server -basepath /Users/ad30343/development /Users/ad30343/development/test-files
    successfully uploaded

    real    0m59.067s
    user    0m5.945s
    sys     0m13.160s

    35% max CPU


#### OSX tar + ssh
    time tar -cf - test-files/ | ssh server@essd-archive-backend-01 "tar -xf - -C sshtest"

    real    1m18.260s
    user    0m6.109s
    sys     0m12.453s

    18% max CPU

#### OSX rsync with compression
    time rsync -az test-files server@essd-archive-backend-01:sshtest/

    real	1m0.190s
    user	0m21.341s
    sys	0m14.735s

    60%+ max CPU

#### scp (no compression)
    time scp -r test-files/ server@essd-archive-backend-01:sshtest/

    10 minutes and still running, I got bored :-)

    1% max CPU