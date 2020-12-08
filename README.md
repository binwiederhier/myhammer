# myhammer

Tool to hammer MySQL, and verify if transactions get lost in fail-overs using semi-sync.

`myhammer run` creates a lot of write-traffic on a table with an auto-increment key and records the last key that MySQL acknowledged. When a failover to a replica host is triggered in MySQL, the last read key can be used to verify that no transaction got lost. For this to work, you need to create enough load so that your replicas will lag, and you need to run semi-sync MySQL replicas with the AFTER_SYNC setting (lossless replication).

## Usage

```
$ myhammer run -p mypassword
...
worker=3 value=3635 key=72666
worker=5 value=3632 key=72675
worker=19 value=3627 key=72664
worker=8 value=3631 key=72668
worker=15 value=3629 key=72669

    ## At this point you kill the source MySQL host

[mysql] 2020/11/26 13:02:56 packets.go:36: unexpected EOF
error: invalid connection, stopping worker 5
[mysql] 2020/11/26 13:02:56 packets.go:36: unexpected EOF
[mysql] 2020/11/26 13:02:56 packets.go:36: unexpected EOF
...
error: invalid connection, stopping worker 17
error: invalid connection, stopping worker 7
[mysql] 2020/11/26 13:02:56 packets.go:36: unexpected EOF
error: invalid connection, stopping worker 2
[mysql] 2020/11/26 13:02:56 packets.go:36: unexpected EOF
error: invalid connection, stopping worker 4

Program exited. Max key = 72675  <<<<<<<<<< This is the interesting value
```

Now you wait for the replicas to catch up and see if the max key was replicated properly.
