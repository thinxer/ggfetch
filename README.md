ggfetch
=======

A simple HTML/Image fetcher based on groupcache, with TTL support.

Deployment
----------

Copy `ggfetch` and `ggfetch.yml` to the target machine, and adjust `ggfetch.yml` according to your usage. You may type `ggfetch -h` to learn about flags.

When deployed as a cluster, you may choose one machine as the master, and make other nodes connect to the master. The nodes will then learn about each other, and will fetch the configuration from the master. Use the `-master` to connect to the master, or use an empty value to become a master. You need to listen on the external IP address when used in a cluster.

When deployed in EC2, you can bind to a special address called `ec2`, and the server will learn its private IPv4 address automatically.

APIs
------

You may call `/{method}?{queries}` directly. Or you may choose to use a simple client, which has additional TTL support. Import `github.com/thinxer/ggfetch/client` and use the `Client` for queries. Doc [here](http://godoc.org/github.com/thinxer/ggfetch/client).

License
-------

MIT License. See LICENSE for details.
