# myhammer

Tool to hammer MySQL, and verify if transactions get lost in fail-overs using semi-sync.

`myhammer run` creates a lot of write-traffic on a table with an auto-increment key and records the last key that MySQL acknowledged. When a failover to a replica host is triggered in MySQL, the last read key can be used to verify that no transaction got lost. For this to work, you need to create enough load so that your replicas will lag, and you need to run semi-sync MySQL replicas with the AFTER_SYNC setting (lossless replication).
