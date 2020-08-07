# fastarchive: tar+gzip over ssh in golang
This utility provides a go implementation of tar over SSH. It uses the io.Writer and io.Reader interfaces
that are chained together much like *nix pipes, resulting in no intermediate buffering or save. <br>

This utility has switched to using github.com/mholt/archive which was found to be more flexible to
switch compression types easily.

Equivalent to
`tar -czf - <dir> | ssh server "tar -xzf - -C <destdir>"`

## Performance
The test transfers about 830M of data with 45000 small files to a remote server over the internet 

#### fastarchive
    time ./fastarchive -server essd-archive-backend-01.wdc.com -user server -destpath sshtest test-files/
    successfully uploaded

    real	0m50.895s
    user	0m8.599s
    sys	0m11.586s


#### OSX tar + ssh
    time tar -czf - test-files/ | ssh server@essd-archive-backend-01.wdc.com "tar -xzf - -C sshtest"

    real	0m58.359s
    user	0m29.945s
    sys	0m12.309s

#### OSX rsync with compression
    time rsync -az test-files/ server@essd-archive-backend-01.wdc.com:sshtest/

    real	0m54.740s
    user	0m21.290s
    sys	0m14.965s

#### scp (no compression)
    time scp -r test-files/ server@essd-archive-backend-01:sshtest/

    10 minutes and still running, I got bored :-)

